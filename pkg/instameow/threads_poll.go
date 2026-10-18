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

package instameow

import (
	"context"
	"slices"
	"strconv"
	"time"

	"github.com/rs/zerolog"

	"go.mau.fi/mautrix-meta/pkg/instameow/slidetypes"
)

// For some threads.com accounts the lightspeed sync stream is closed by the server right after
// it's established (the web app keeps reconnecting too). Messages still work over GraphQL, so the
// client polls the inbox while the stream is down and turns changes into the same deltas the
// stream would deliver.

var (
	// ThreadsPollInterval is how often the inbox is polled while the realtime stream is down.
	ThreadsPollInterval = 15 * time.Second
	// threadsPollStartDelay gives the realtime stream a chance to connect before polling starts.
	threadsPollStartDelay = 20 * time.Second
	// threadsPollThreadCount is how many recent threads each poll checks.
	threadsPollThreadCount = 15
	// threadsPollMessageCount is how many recent messages are compared in a changed thread.
	threadsPollMessageCount = 20
	// threadsStreamRetryInterval is the reconnect interval for the realtime stream while polling.
	threadsStreamRetryInterval = 5 * time.Minute
)

type polledMessage struct {
	timestampMS int64
	text        string
	hasText     bool
	reactions   map[string]*slidetypes.Reaction
}

type threadsPollState struct {
	startMS  int64
	activity map[string]int64
	messages map[string]map[string]polledMessage
	receipts map[string]map[int64]int64
}

func threadActivityMS(thread *slidetypes.ThreadsThread) int64 {
	ts := thread.TimestampMS.UnixMilli()
	if ctx := thread.LatestSnippetActivityContext; ctx != nil {
		ts = max(ts, ctx.TimestampMS.UnixMilli())
	}
	return ts
}

func digestMessage(msg *slidetypes.Message) polledMessage {
	pm := polledMessage{
		timestampMS: msg.TimestampMS.UnixMilli(),
		reactions:   make(map[string]*slidetypes.Reaction, len(msg.Reactions)),
	}
	if text, ok := msg.Content.Content.(*slidetypes.MessageContentText); ok {
		pm.text, pm.hasText = text.TextBody, true
	}
	for _, react := range msg.Reactions {
		if react != nil {
			pm.reactions[reactionKey(react)] = react
		}
	}
	return pm
}

func reactionKey(react *slidetypes.Reaction) string {
	return react.Reaction + "/" + strconv.FormatInt(react.SenderFBID, 10)
}

// runThreadsPoller polls the inbox whenever the realtime stream isn't connected. It returns when
// ctx is canceled.
func (c *Client) runThreadsPoller(ctx context.Context) {
	log := zerolog.Ctx(ctx).With().Str("component", "threads poller").Logger()
	ctx = log.WithContext(ctx)
	state := &threadsPollState{startMS: time.Now().UnixMilli()}
	select {
	case <-ctx.Done():
		return
	case <-time.After(threadsPollStartDelay):
	}
	ticker := time.NewTicker(ThreadsPollInterval)
	defer ticker.Stop()
	for {
		if c.connected.IsSet() {
			if c.threadsPolling.Swap(false) {
				log.Info().Msg("Realtime stream connected, pausing inbox polling")
			}
		} else if err := c.pollThreads(ctx, state); err != nil {
			log.Warn().Err(err).Msg("Failed to poll Threads inbox")
		} else if !c.threadsPolling.Swap(true) {
			log.Info().Dur("interval", ThreadsPollInterval).Msg("Realtime stream unavailable, polling the inbox instead")
			if err = c.eventHandler(ctx, &slidetypes.Connected{}); err != nil {
				log.Err(err).Msg("Failed to dispatch connected event for polling")
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (c *Client) dispatchPolledDelta(ctx context.Context, threadID, typeName string, data slidetypes.DeltaEvent) error {
	return c.eventHandler(ctx, &slidetypes.Delta{TypeName: typeName, ThreadIGID: threadID, Data: data})
}

func (c *Client) pollThreads(ctx context.Context, state *threadsPollState) error {
	list, err := c.threadsThreadList(ctx, "INBOX", "", threadsPollThreadCount)
	if err != nil {
		return err
	}
	ownFBID := c.threadsOwnFBID()
	baseline := state.activity == nil
	if baseline {
		state.activity = make(map[string]int64)
		state.messages = make(map[string]map[string]polledMessage)
		state.receipts = make(map[string]map[int64]int64)
	}
	for _, edge := range list.Mailbox.ThreadsByFolder.Edges {
		thread := edge.Node
		if thread == nil || thread.ID == "" {
			continue
		}
		if err = c.pollReadReceipts(ctx, state, thread, ownFBID, baseline); err != nil {
			return err
		}
		activity := threadActivityMS(thread)
		prev, known := state.activity[thread.ID]
		if baseline {
			state.activity[thread.ID] = activity
			continue
		} else if known && activity <= prev {
			continue
		}
		since := prev
		if !known {
			since = state.startMS
		}
		if err = c.pollThreadMessages(ctx, state, thread.ID, since); err != nil {
			return err
		}
		state.activity[thread.ID] = activity
	}
	return nil
}

func (c *Client) pollReadReceipts(ctx context.Context, state *threadsPollState, thread *slidetypes.ThreadsThread, ownFBID int64, baseline bool) error {
	receipts := state.receipts[thread.ID]
	if receipts == nil {
		receipts = make(map[int64]int64)
		state.receipts[thread.ID] = receipts
	}
	for _, rr := range thread.ReadReceipts {
		watermark := rr.WatermarkTimestampMS.UnixMilli()
		prev, known := receipts[rr.ParticipantFBID]
		receipts[rr.ParticipantFBID] = max(prev, watermark)
		if baseline || !known || watermark <= prev || rr.ParticipantFBID == ownFBID {
			continue
		}
		err := c.dispatchPolledDelta(ctx, thread.ID, "SlideUQPPReadReceipt", &slidetypes.ReadReceiptEvent{ReadReceipt: rr})
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) pollThreadMessages(ctx context.Context, state *threadsPollState, threadID string, sinceMS int64) error {
	thread, err := c.threadsMessageList(ctx, threadID, nil, threadsPollMessageCount)
	if err != nil {
		return err
	}
	var msgs []*slidetypes.Message
	if thread.Messages != nil {
		for _, edge := range thread.Messages.Edges {
			if edge.Node != nil && edge.Node.MessageID != "" {
				msgs = append(msgs, &edge.Node.Message)
			}
		}
	}
	slices.SortFunc(msgs, func(a, b *slidetypes.Message) int {
		return a.TimestampMS.Compare(b.TimestampMS.Time)
	})
	cache := state.messages[threadID]
	fresh := make(map[string]polledMessage, len(msgs))
	for _, msg := range msgs {
		digest := digestMessage(msg)
		fresh[msg.MessageID] = digest
		old, had := cache[msg.MessageID]
		if !had {
			if digest.timestampMS <= sinceMS {
				continue
			}
			typeName, data := "SlideUQPPNewMessage", slidetypes.DeltaEvent(&slidetypes.NewMessageEvent{Message: msg})
			if _, isAdmin := msg.Content.Content.(*slidetypes.MessageContentAdminText); isAdmin {
				typeName, data = "SlideUQPPAdminTextMessage", &slidetypes.AdminMessageEvent{Message: msg}
			}
			if err = c.dispatchPolledDelta(ctx, threadID, typeName, data); err != nil {
				return err
			}
			continue
		}
		if digest.hasText && old.hasText && digest.text != old.text {
			edit := &slidetypes.EditMessageEvent{MessageID: msg.MessageID, TextBody: digest.text}
			edit.SlideEditHistoryEntry.TimestampMS = msg.TimestampMS
			if err = c.dispatchPolledDelta(ctx, threadID, "SlideUQPPEditMessage", edit); err != nil {
				return err
			}
		}
		if err = c.diffReactions(ctx, threadID, msg.MessageID, old, digest); err != nil {
			return err
		}
	}
	if cache != nil && len(msgs) > 0 {
		oldest := msgs[0].TimestampMS.UnixMilli()
		for id, old := range cache {
			if _, still := fresh[id]; !still && old.timestampMS >= oldest {
				if err = c.dispatchPolledDelta(ctx, threadID, "SlideUQPPDeleteMessage", &slidetypes.DeleteMessageEvent{MessageID: id}); err != nil {
					return err
				}
			}
		}
	}
	state.messages[threadID] = fresh
	return nil
}

func (c *Client) diffReactions(ctx context.Context, threadID, messageID string, old, cur polledMessage) error {
	oldByKey, curByKey := old.reactions, cur.reactions
	for key, react := range curByKey {
		if _, had := oldByKey[key]; !had {
			err := c.dispatchPolledDelta(ctx, threadID, "SlideUQPPCreateReaction", &slidetypes.CreateReactionEvent{MessageID: messageID, Reaction: *react})
			if err != nil {
				return err
			}
		}
	}
	for key, react := range oldByKey {
		if _, still := curByKey[key]; !still {
			err := c.dispatchPolledDelta(ctx, threadID, "SlideUQPPDeleteReaction", &slidetypes.DeleteReactionEvent{MessageID: messageID, Reaction: *react})
			if err != nil {
				return err
			}
		}
	}
	return nil
}
