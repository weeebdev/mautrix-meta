// mautrix-meta - A Matrix-Facebook Messenger and Instagram DM puppeting bridge.
// Copyright (C) 2026 Tulir Asokan
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package igconnector

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"os"
	"strings"
	"time"

	"go.mau.fi/util/dbutil"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/commands"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/bridgev2/simplevent"
	"maunium.net/go/mautrix/bridgev2/status"
	"maunium.net/go/mautrix/event"

	"go.mau.fi/mautrix-meta/pkg/metaid"
)

// Optional privacy features, enabled with IGConnector.PrivacyFeatures:
//
//   - invisible: don't send read receipts or typing notifications
//   - pause/resume: disconnect a login until resumed, across restarts
//   - keep-deleted: messages others delete stay in Matrix, marked as deleted
//   - show-edits: edited messages list their earlier versions underneath
//
// keep-deleted and show-edits need the content of other people's messages, which is stored in
// message metadata encrypted with AES-256-GCM, bound to the message and part ID. The key comes
// from CONTENT_ENCRYPTION_KEY (standard base64 of 32 bytes) and never touches the database.
// Without the key nothing is stored. The user's own messages are never stored.

const contentKeyEnv = "CONTENT_ENCRYPTION_KEY"

// maxEditHistory caps how many earlier versions of a message are kept.
const maxEditHistory = 10

type storedContent struct {
	Content     *event.MessageEventContent `json:"c"`
	EditHistory []string                   `json:"h,omitempty"`
}

type contentSealer struct {
	aead cipher.AEAD
}

func newContentSealerFromEnv() (*contentSealer, error) {
	raw := strings.TrimSpace(os.Getenv(contentKeyEnv))
	if raw == "" {
		return nil, nil
	}
	return newContentSealer(raw)
}

func newContentSealer(b64Key string) (*contentSealer, error) {
	key, err := base64.StdEncoding.DecodeString(b64Key)
	if err != nil {
		return nil, fmt.Errorf("%s is not valid base64: %w", contentKeyEnv, err)
	} else if len(key) != 32 {
		return nil, fmt.Errorf("%s must be 32 bytes, got %d", contentKeyEnv, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &contentSealer{aead: aead}, nil
}

// Seal encrypts sc, binding it to the given ID so it can't be moved to another message.
func (s *contentSealer) Seal(id string, sc *storedContent) (string, error) {
	plain, err := json.Marshal(sc)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(s.aead.Seal(nonce, nonce, plain, []byte(id))), nil
}

func (s *contentSealer) Open(id, sealed string) (*storedContent, error) {
	data, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return nil, err
	}
	ns := s.aead.NonceSize()
	if len(data) < ns {
		return nil, errors.New("sealed content too short")
	}
	plain, err := s.aead.Open(nil, data[:ns], data[ns:], []byte(id))
	if err != nil {
		return nil, err
	}
	var sc storedContent
	return &sc, json.Unmarshal(plain, &sc)
}

func sealID(messageID networkid.MessageID, partID networkid.PartID) string {
	return string(messageID) + "/" + string(partID)
}

// startPrivacy loads the content key, compacts the database and registers the bot commands.
func (ic *IGConnector) startPrivacy(ctx context.Context) error {
	if !ic.PrivacyFeatures {
		return nil
	}
	var err error
	if ic.sealer, err = newContentSealerFromEnv(); err != nil {
		return err
	} else if ic.sealer == nil {
		ic.Bridge.Log.Warn().Msg(contentKeyEnv + " not set: message content won't be stored, so keep-deleted and show-edits are degraded")
	}
	ic.vacuumSQLite(ctx)
	if proc, ok := ic.Bridge.Commands.(*commands.Processor); ok {
		proc.AddHandlers(cmdInvisible, cmdKeepDeleted, cmdShowEdits, cmdPause, cmdResume)
	}
	return nil
}

// vacuumSQLite rebuilds a SQLite database so removed content doesn't linger in free pages, then
// truncates the WAL. It runs on every start. For deletes between restarts, set _secure_delete=on
// in the database URI.
func (ic *IGConnector) vacuumSQLite(ctx context.Context) {
	db := ic.Bridge.DB.Database
	if db.Dialect != dbutil.SQLite {
		return
	}
	log := ic.Bridge.Log.With().Str("action", "vacuum database").Logger()
	_, err := db.Exec(ctx, "PRAGMA secure_delete=ON")
	if err == nil {
		_, err = db.Exec(ctx, "VACUUM")
	}
	if err == nil {
		_, err = db.Exec(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
	}
	if err != nil {
		log.Err(err).Msg("Failed to vacuum database")
	} else {
		log.Debug().Msg("Vacuumed database")
	}
}

func (ic *IGClient) privacyEnabled() bool {
	return ic.Main.PrivacyFeatures
}

func (ic *IGClient) isOwnFBID(fbid int64) bool {
	return metaid.MakeUserLoginID(fbid) == ic.UserLogin.ID
}

// sealConverted stores the encrypted content of each part of someone else's message.
func (ic *IGClient) sealConverted(msgID networkid.MessageID, senderFBID int64, cm *bridgev2.ConvertedMessage) {
	if !ic.privacyEnabled() || ic.Main.sealer == nil || cm == nil || ic.isOwnFBID(senderFBID) {
		return
	}
	for _, part := range cm.Parts {
		if part == nil || part.Content == nil {
			continue
		}
		sealed, err := ic.Main.sealer.Seal(sealID(msgID, part.ID), &storedContent{Content: part.Content})
		if err != nil {
			ic.UserLogin.Log.Err(err).Msg("Failed to encrypt message content")
			continue
		}
		meta, _ := part.DBMetadata.(*metaid.MessageMetadata)
		if meta == nil {
			meta = &metaid.MessageMetadata{}
			part.DBMetadata = meta
		}
		meta.Sealed = sealed
	}
}

// openContent decrypts the stored content of a message part, or returns nil.
func (ic *IGClient) openContent(part *database.Message) *storedContent {
	meta, ok := part.Metadata.(*metaid.MessageMetadata)
	if !ok || meta.Sealed == "" || ic.Main.sealer == nil {
		return nil
	}
	sc, err := ic.Main.sealer.Open(sealID(part.ID, part.PartID), meta.Sealed)
	if err != nil {
		ic.UserLogin.Log.Warn().Err(err).Msg("Failed to decrypt stored message content")
		return nil
	}
	return sc
}

// applyEditHistory updates the stored content of an edited part and returns the content to show.
func (ic *IGClient) applyEditHistory(part *database.Message, content *event.MessageEventContent) *event.MessageEventContent {
	if !ic.privacyEnabled() {
		return content
	}
	stored := ic.openContent(part)
	if stored == nil {
		return content
	}
	history := stored.EditHistory
	if stored.Content != nil && stored.Content.Body != content.Body {
		history = append([]string{stored.Content.Body}, history...)
		if len(history) > maxEditHistory {
			history = history[:maxEditHistory]
		}
	}
	if sealed, err := ic.Main.sealer.Seal(sealID(part.ID, part.PartID), &storedContent{Content: content, EditHistory: history}); err == nil {
		part.Metadata.(*metaid.MessageMetadata).Sealed = sealed
	}
	if ic.LoginMeta.ShowEditsEnabled() && len(history) > 0 {
		return withEditHistory(content, history)
	}
	return content
}

// withEditHistory returns a copy of content with previous versions appended, newest first.
func withEditHistory(orig *event.MessageEventContent, history []string) *event.MessageEventContent {
	c := *orig
	formatted := c.FormattedBody
	if c.Format != event.FormatHTML || formatted == "" {
		formatted = strings.ReplaceAll(html.EscapeString(c.Body), "\n", "<br/>")
	}
	var plain, rich strings.Builder
	plain.WriteString(c.Body)
	rich.WriteString(formatted)
	for _, old := range history {
		plain.WriteString("\n✏️ was: " + old)
		rich.WriteString(`<br/><font color="#888888"><sub>✏️ was: ` +
			strings.ReplaceAll(html.EscapeString(old), "\n", "<br/>") + "</sub></font>")
	}
	c.Body = plain.String()
	c.Format = event.FormatHTML
	c.FormattedBody = rich.String()
	return &c
}

func (ic *IGClient) deletedLabel() string {
	return "🗑 deleted in " + ic.Main.GetName().DisplayName
}

// deletedContent returns a copy of content marked as deleted: text is struck through, media keeps
// its file with a marked caption.
func deletedContent(orig *event.MessageEventContent, label string) *event.MessageEventContent {
	c := *orig
	c.Mentions = &event.Mentions{}
	switch c.MsgType {
	case event.MsgText, event.MsgNotice, event.MsgEmote:
		formatted := c.FormattedBody
		if c.Format != event.FormatHTML || formatted == "" {
			formatted = strings.ReplaceAll(html.EscapeString(c.Body), "\n", "<br/>")
		}
		c.Format = event.FormatHTML
		c.FormattedBody = "<del>" + formatted + "</del><br/><em>" + html.EscapeString(label) + "</em>"
		c.Body = "~~" + c.Body + "~~\n" + label
	default:
		name := c.FileName
		if name == "" {
			name = c.Body
		}
		c.FileName = name
		c.Body = name + " (" + label + ")"
		c.Format, c.FormattedBody = "", ""
	}
	return &c
}

// keepDeleted handles a remote delete when keep-deleted is on. It returns false if the message
// should be removed as usual: the user's own messages, and messages that are already gone.
func (ic *IGClient) keepDeleted(ctx context.Context, portalKey networkid.PortalKey, msgID networkid.MessageID) (bridgev2.EventHandlingResult, bool) {
	if !ic.privacyEnabled() || !ic.LoginMeta.KeepDeletedEnabled() {
		return bridgev2.EventHandlingResult{}, false
	}
	parts, err := ic.Main.Bridge.DB.Message.GetAllPartsByID(ctx, portalKey.Receiver, msgID)
	if err != nil || len(parts) == 0 {
		return bridgev2.EventHandlingResult{}, false
	}
	stored := false
	for _, part := range parts {
		if part.SenderID == networkid.UserID(ic.UserLogin.ID) || ic.isOwnFBID(metaid.ParseUserID(part.SenderID)) {
			// Messages the user deletes are always removed.
			return bridgev2.EventHandlingResult{}, false
		}
		if meta, ok := part.Metadata.(*metaid.MessageMetadata); ok && meta.Deleted {
			// Already marked as deleted.
			return bridgev2.EventHandlingResultIgnored, true
		}
		if ic.openContent(part) != nil {
			stored = true
		}
	}
	if !stored {
		return ic.replyDeleted(portalKey, msgID), true
	}
	label := ic.deletedLabel()
	return ic.UserLogin.QueueRemoteEvent(&simplevent.Message[networkid.MessageID]{
		EventMeta: simplevent.EventMeta{
			Type:      bridgev2.RemoteEventEdit,
			PortalKey: portalKey,
			Sender:    bridgev2.EventSender{ForceEditOrigSender: true},
			Timestamp: time.Now(),
		},
		Data:          msgID,
		TargetMessage: msgID,
		ConvertEditFunc: func(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, existing []*database.Message, _ networkid.MessageID) (*bridgev2.ConvertedEdit, error) {
			edit := &bridgev2.ConvertedEdit{}
			for _, part := range existing {
				meta, ok := part.Metadata.(*metaid.MessageMetadata)
				stored := ic.openContent(part)
				if !ok || stored == nil || stored.Content == nil || meta.Deleted {
					continue
				}
				content := deletedContent(stored.Content, label)
				if ic.LoginMeta.ShowEditsEnabled() && len(stored.EditHistory) > 0 {
					content = withEditHistory(content, stored.EditHistory)
				}
				edit.ModifiedParts = append(edit.ModifiedParts, &bridgev2.ConvertedEditPart{
					Part:    part,
					Type:    event.EventMessage,
					Content: content,
				})
				meta.Deleted = true
			}
			if len(edit.ModifiedParts) == 0 {
				return nil, fmt.Errorf("%w: nothing to mark as deleted", bridgev2.ErrIgnoringRemoteEvent)
			}
			return edit, nil
		},
	}), true
}

// replyDeleted posts a notice replying to a deleted message whose content wasn't stored.
func (ic *IGClient) replyDeleted(portalKey networkid.PortalKey, msgID networkid.MessageID) bridgev2.EventHandlingResult {
	label := "🗑 This message was " + strings.TrimPrefix(ic.deletedLabel(), "🗑 ")
	return ic.UserLogin.QueueRemoteEvent(&simplevent.Message[networkid.MessageID]{
		EventMeta: simplevent.EventMeta{
			Type:      bridgev2.RemoteEventMessage,
			PortalKey: portalKey,
			Timestamp: time.Now(),
		},
		Data: msgID,
		ID:   networkid.MessageID("deleted-" + string(msgID)),
		ConvertMessageFunc: func(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, target networkid.MessageID) (*bridgev2.ConvertedMessage, error) {
			return &bridgev2.ConvertedMessage{
				ReplyTo: &networkid.MessageOptionalPartID{MessageID: target},
				Parts: []*bridgev2.ConvertedMessagePart{{
					Type:    event.EventMessage,
					Content: &event.MessageEventContent{MsgType: event.MsgNotice, Body: label},
				}},
			}, nil
		},
	})
}

// isInvisible returns true if read receipts and typing notifications shouldn't be sent.
func (ic *IGClient) isInvisible() bool {
	return ic.privacyEnabled() && ic.LoginMeta.Invisible
}

// isPaused returns true if the login is paused.
func (ic *IGClient) isPaused() bool {
	return ic.privacyEnabled() && ic.LoginMeta.Paused
}

func (ic *IGClient) sendPausedState() {
	ic.UserLogin.BridgeState.Send(status.BridgeState{
		StateEvent: status.StateTransientDisconnect,
		Error:      "meta-paused",
		Message:    "Bridge is paused. Send `resume` to the bridge bot to reconnect.",
	})
}

func onOff(v bool) string {
	if v {
		return "on"
	}
	return "off"
}

func boolArg(args []string) (value, ok bool) {
	if len(args) == 0 {
		return false, false
	}
	switch strings.ToLower(args[0]) {
	case "on", "yes", "true", "1", "enable":
		return true, true
	case "off", "no", "false", "0", "disable":
		return false, true
	}
	return false, false
}

func settingCommand(name, description, explain string, get func(*metaid.UserLoginMetadata) bool, set func(*metaid.UserLoginMetadata, bool)) *commands.FullHandler {
	return &commands.FullHandler{
		Func: func(ce *commands.Event) {
			login := ce.User.GetDefaultLogin()
			if login == nil {
				ce.Reply("You're not logged in")
				return
			}
			meta := login.Metadata.(*metaid.UserLoginMetadata)
			value, ok := boolArg(ce.Args)
			if !ok {
				ce.Reply("%s is **%s**. Usage: `%s on|off`\n\n%s", name, onOff(get(meta)), name, explain)
				return
			}
			set(meta, value)
			if err := login.Save(ce.Ctx); err != nil {
				ce.Reply("Failed to save setting: %v", err)
				return
			}
			ce.Reply("%s is now **%s**", name, onOff(value))
		},
		Name: name,
		Help: commands.HelpMeta{
			Section:     commands.HelpSectionGeneral,
			Description: description,
			Args:        "[on|off]",
		},
		RequiresLogin: true,
	}
}

var cmdInvisible = settingCommand("invisible",
	"Don't send read receipts or typing notifications",
	"When on, reading or typing in Matrix isn't reported to the other side, and chats stay unread there.",
	func(m *metaid.UserLoginMetadata) bool { return m.Invisible },
	func(m *metaid.UserLoginMetadata, v bool) { m.Invisible = v })

var cmdKeepDeleted = settingCommand("keep-deleted",
	"Keep messages in Matrix when others delete them",
	"When on, messages others delete stay in Matrix, struck through and marked 🗑, with attachments kept. "+
		"Messages you delete are always removed. Needs "+contentKeyEnv+" on the bridge.",
	func(m *metaid.UserLoginMetadata) bool { return m.KeepDeletedEnabled() },
	func(m *metaid.UserLoginMetadata, v bool) { m.KeepDeleted = &v })

var cmdShowEdits = settingCommand("show-edits",
	"Show previous versions under edited messages",
	"When on, edited messages list their earlier versions underneath (\"✏️ was: …\"), newest first. "+
		"Needs "+contentKeyEnv+" on the bridge.",
	func(m *metaid.UserLoginMetadata) bool { return m.ShowEditsEnabled() },
	func(m *metaid.UserLoginMetadata, v bool) { m.ShowEdits = &v })

func pauseCommand(name, description string, pause bool) *commands.FullHandler {
	return &commands.FullHandler{
		Func: func(ce *commands.Event) {
			login := ce.User.GetDefaultLogin()
			if login == nil {
				ce.Reply("You're not logged in")
				return
			}
			meta := login.Metadata.(*metaid.UserLoginMetadata)
			if meta.Paused == pause {
				ce.Reply("Bridge is already %s", map[bool]string{true: "paused", false: "running"}[pause])
				return
			}
			meta.Paused = pause
			if err := login.Save(ce.Ctx); err != nil {
				ce.Reply("Failed to save: %v", err)
				return
			}
			client := login.Client.(*IGClient)
			if pause {
				client.Disconnect()
				client.sendPausedState()
				ce.Reply("Paused: disconnected. Nothing is sent or received until you send `resume`, including after restarts.")
			} else {
				go client.Connect(login.Log.WithContext(client.Main.Bridge.BackgroundCtx))
				ce.Reply("Resuming: reconnecting. Messages sent while paused are fetched by the next sync.")
			}
		},
		Name: name,
		Help: commands.HelpMeta{
			Section:     commands.HelpSectionGeneral,
			Description: description,
		},
		RequiresLogin: true,
	}
}

var cmdPause = pauseCommand("pause", "Disconnect and stop bridging until `resume` (survives restarts)", true)
var cmdResume = pauseCommand("resume", "Reconnect after `pause`", false)
