package metaid

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/json"
	"sync/atomic"

	"go.mau.fi/util/exerrors"
	"go.mau.fi/util/random"
	waTypes "go.mau.fi/whatsmeow/types"
	"maunium.net/go/mautrix/bridgev2/networkid"

	"go.mau.fi/mautrix-meta/pkg/messagix/cookies"
	"go.mau.fi/mautrix-meta/pkg/messagix/pushcrypto"
	"go.mau.fi/mautrix-meta/pkg/messagix/securehistory"
	"go.mau.fi/mautrix-meta/pkg/messagix/table"
	"go.mau.fi/mautrix-meta/pkg/messagix/types"
)

type MessageMetadata struct {
	EditTimestamp int64 `json:"edit_timestamp,omitempty"`
	// Default direct media meta. Used for blob media, whatsapp media and old XMA media
	DirectMediaMeta json.RawMessage `json:"direct_media_meta,omitempty"`
	// New XMA direct media meta.
	XMADirectMediaMeta json.RawMessage `json:"xma_media_meta,omitempty"`
	// Metadata for postponed fetching of XMA media.
	XMAFetchMeta *XMAFetchMeta `json:"xma_fetch_meta,omitempty"`
	XMAFetched   bool          `json:"xma_fetched,omitempty"`

	// Privacy features (igconnector PrivacyFeatures): the encrypted bridged content and previous
	// versions of someone else's message, and whether it was deleted remotely and kept.
	Sealed  string `json:"sealed,omitempty"`
	Deleted bool   `json:"deleted,omitempty"`
}

type XMAFetchMeta struct {
	TargetURL string `json:"target_url,omitempty"`
	TargetID  int64  `json:"target_id,omitempty"`
	IsStory   bool   `json:"is_story,omitempty"`
}

type GhostMetadata struct {
	Username string `json:"username,omitempty"`
	IGID     string `json:"igid,omitempty"`
}

type UserLoginMetadata struct {
	Platform               types.Platform                `json:"platform"`
	Cookies                *cookies.Cookies              `json:"cookies"`
	WADeviceID             uint16                        `json:"wa_device_id,omitempty"`
	PushKeys               *pushcrypto.PushKeys          `json:"push_keys,omitempty"`
	LoginUA                string                        `json:"login_ua,omitempty"`
	IGID                   string                        `json:"igid,omitempty"`
	NativeSession          *types.NativeSession          `json:"native_session,omitempty"`
	NativePushKeys         *pushcrypto.NativePushKeys    `json:"native_push_keys,omitempty"`
	InstagramNativeSession *types.InstagramNativeSession `json:"instagram_native_session,omitempty"`
	HistoryDevice          *securehistory.Device         `json:"history_device,omitempty"`

	// Thread backfill state
	BackfillCompleted bool `json:"backfill_completed,omitempty"`

	// Privacy settings (igconnector PrivacyFeatures), toggled with bot commands
	Invisible   bool  `json:"invisible,omitempty"`
	Paused      bool  `json:"paused,omitempty"`
	KeepDeleted *bool `json:"keep_deleted,omitempty"`
	ShowEdits   *bool `json:"show_edits,omitempty"`
}

// KeepDeletedEnabled returns the keep-deleted setting, which defaults to on.
func (m *UserLoginMetadata) KeepDeletedEnabled() bool {
	return m.KeepDeleted == nil || *m.KeepDeleted
}

// ShowEditsEnabled returns the show-edits setting, which defaults to on.
func (m *UserLoginMetadata) ShowEditsEnabled() bool {
	return m.ShowEdits == nil || *m.ShowEdits
}

func (m *UserLoginMetadata) CopyFrom(other any) {
	updated := *other.(*UserLoginMetadata)
	if m.Platform.IsMessenger() && updated.Platform.IsMessenger() && m.HistoryDevice != nil {
		updated.HistoryDevice = m.HistoryDevice
	}
	// Privacy settings survive a re-login (except pause: logging in again resumes).
	updated.Invisible = m.Invisible
	if updated.KeepDeleted == nil {
		updated.KeepDeleted = m.KeepDeleted
	}
	if updated.ShowEdits == nil {
		updated.ShowEdits = m.ShowEdits
	}
	*m = updated
}

func (m *UserLoginMetadata) GeneratePushKeys() {
	privateKey := exerrors.Must(ecdh.P256().GenerateKey(rand.Reader))
	m.PushKeys = &pushcrypto.PushKeys{
		P256DH:  privateKey.Public().(*ecdh.PublicKey).Bytes(),
		Auth:    random.Bytes(16),
		Private: privateKey.Bytes(),
	}
}

type PortalMetadata struct {
	ThreadType     table.ThreadType `json:"thread_type"`
	WhatsAppServer string           `json:"whatsapp_server,omitempty"`

	// FBThreadKey is the original Facebook thread key for hybrid threads. The portal is keyed by the
	// WhatsApp thread JID (the result of MapWhatsAppThreadKey), but some tasks (e.g. accepting a
	// message request or deleting the legacy thread) need the original Facebook key instead.
	FBThreadKey int64 `json:"fb_thread_key,omitempty"`

	IGID       string `json:"igid,omitempty"`
	IGThreadID string `json:"ig_thread_id,omitempty"`

	EphemeralSettingTimestamp int64 `json:"ephemeral_setting_timestamp,omitempty"`

	FetchAttempted atomic.Bool `json:"-"`
}

func (meta *PortalMetadata) JID(id networkid.PortalID) waTypes.JID {
	jid := ParseWAPortalID(id, meta.WhatsAppServer)
	if jid.Server == "" {
		switch meta.ThreadType {
		case table.ENCRYPTED_OVER_WA_GROUP:
			jid.Server = waTypes.GroupServer
		//case table.ENCRYPTED_OVER_WA_ONE_TO_ONE:
		//	jid.Server = waTypes.DefaultUserServer
		default:
			jid.Server = waTypes.MessengerServer
		}
	}
	return jid
}
