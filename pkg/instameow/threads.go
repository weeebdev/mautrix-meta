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
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strconv"
	"time"

	"go.mau.fi/mautrix-meta/pkg/instameow/slidetypes"
	"go.mau.fi/mautrix-meta/pkg/messagix/methods"
)

// threads.com runs direct messages on the same Slide backend as Instagram web, but with its own
// GraphQL operations (WmiSlide* and Barcelona*). The methods here implement the Instagram client
// calls for threads.com and convert the responses into the Instagram types.

// ErrNotSupportedOnThreads is returned for Instagram features threads.com doesn't have.
var ErrNotSupportedOnThreads = errors.New("not supported on Threads")

// threadsMailboxPageSize is how many threads the initial inbox load fetches.
const threadsMailboxPageSize = 20

// threadsMessagePageSize is how many messages a thread fetch includes.
const threadsMessagePageSize = 20

// threadsRelayProviders are the relay provider variables threads.com expects. The Micg migration
// providers are off so that messages come back in the Slide model Instagram uses.
var threadsRelayProviders = map[string]any{
	"__relay_internal__pv__BarcelonaIsMicgThreadMigrationEnabledrelayprovider":            false,
	"__relay_internal__pv__BarcelonaIsMicgUserMigrationEnabledrelayprovider":              false,
	"__relay_internal__pv__BarcelonaIsMicgMessageMigrationEnabledrelayprovider":           false,
	"__relay_internal__pv__BarcelonaMessagesHasLiveChatMessagingrelayprovider":            false,
	"__relay_internal__pv__BarcelonaHasWebFaviconsrelayprovider":                          false,
	"__relay_internal__pv__BarcelonaMessagesShouldIncludeNotificationSenderrelayprovider": false,
	"__relay_internal__pv__BarcelonaSnapshotNumberOfMessagesrelayprovider":                threadsMessagePageSize,
	"__relay_internal__pv__BarcelonaIsInternalUserrelayprovider":                          false,
	"__relay_internal__pv__BarcelonaIsLoggedInrelayprovider":                              true,
	"__relay_internal__pv__BarcelonaIsCrawlerrelayprovider":                               false,
	"__relay_internal__pv__BarcelonaMessagingHasMetaAIBotrelayprovider":                   false,
	"__relay_internal__pv__BarcelonaMessagesHasFastFollowsrelayprovider":                  false,
	"__relay_internal__pv__BarcelonaMessagesHasSnippetUnreadMessageCountrelayprovider":    false,
	"__relay_internal__pv__BarcelonaMessagesHasMRUnreadCountEnabledrelayprovider":         false,
}

// threadsDeltaVariables are the graphql_variables sent in the lightspeed sync params. The web app
// turns the Micg providers on; they're off here so deltas use the Slide message model.
var threadsDeltaVariables = func() string {
	data, _ := json.Marshal(map[string]bool{
		"__relay_internal__pv__BarcelonaIsMicgMessageMigrationEnabledrelayprovider": true,
		"__relay_internal__pv__BarcelonaIsMicgUserMigrationEnabledrelayprovider":    true,
		"__relay_internal__pv__BarcelonaIsMicgThreadMigrationEnabledrelayprovider":  true,
		"__relay_internal__pv__BarcelonaMessagesHasLiveChatMessagingrelayprovider":  false,
		"__relay_internal__pv__BarcelonaHasWebFaviconsrelayprovider":                false,
	})
	return string(data)
}()

func threadsVars(vars map[string]any) map[string]any {
	out := maps.Clone(threadsRelayProviders)
	maps.Copy(out, vars)
	return out
}

func threadsOTID(otid string) string {
	if otid != "" {
		return otid
	}
	return strconv.FormatInt(methods.GenerateEpochID(), 10)
}

func threadsQuery[T any](ctx context.Context, c *Client, name string, vars map[string]any) (T, error) {
	return makeGraphQLRequest[T](ctx, c, name, threadsVars(vars), true)
}

// applyThreadsPageConfig maps the threads.com page configs onto the fields the Instagram code
// reads: PolarisViewer from BarcelonaSharedData and the IGD device ID from MqttWebDeviceID.
func (c *Client) applyThreadsPageConfig() {
	table := c.configs.BrowserConfigTable
	if table.BarcelonaSharedData.Viewer.ID != "" {
		table.PolarisViewer = table.BarcelonaSharedData.AsPolarisViewer()
	}
	if table.IGDMqttWebDeviceID.ClientID == "" {
		table.IGDMqttWebDeviceID.ClientID = table.MqttWebDeviceID.ClientID
	}
}

// threadsOwnFBID is the viewer's messaging FBID, which threads.com uses as sender_fbid of the
// viewer's own messages.
func (c *Client) threadsOwnFBID() int64 {
	if c == nil || c.configs == nil {
		return 0
	}
	fbid, _ := strconv.ParseInt(c.configs.BrowserConfigTable.BarcelonaSharedData.Viewer.InteropMessagingUserFBID, 10, 64)
	return fbid
}

// threadsViewer is the logged-in user as an Instagram User.
func (c *Client) threadsViewer() *slidetypes.User {
	viewer := c.configs.BrowserConfigTable.BarcelonaSharedData.Viewer
	return &slidetypes.User{
		InteropMessagingUserFBID: c.threadsOwnFBID(),
		Username:                 viewer.Username,
		ID:                       viewer.ID,
		PK:                       viewer.ID,
		ProfilePicURL:            viewer.ProfilePictureURL,
	}
}

func (c *Client) threadsMailboxSetup(ctx context.Context) (*slidetypes.ThreadsMailboxSetup, error) {
	resp, err := threadsQuery[*slidetypes.ThreadsMailboxSetup](ctx, c, "BarcelonaMessagesSetup_MailboxQuery", map[string]any{
		"device_id_for_iris_subscription": c.configs.BrowserConfigTable.IGDMqttWebDeviceID.ClientID,
	})
	if err != nil {
		return nil, err
	} else if resp == nil || resp.Mailbox.ID == "" {
		return nil, ErrMailboxMissing
	}
	c.threadsMailboxID = resp.Mailbox.ID
	return resp, nil
}

func (c *Client) threadsThreadList(ctx context.Context, folder, cursor string, count int) (*slidetypes.ThreadsThreadListResponse, error) {
	if c.threadsMailboxID == "" {
		if _, err := c.threadsMailboxSetup(ctx); err != nil {
			return nil, err
		}
	}
	if folder == "" {
		folder = "INBOX"
	}
	var after any
	if cursor != "" {
		after = cursor
	}
	return threadsQuery[*slidetypes.ThreadsThreadListResponse](ctx, c, "BarcelonaMessagesThreadListPaginationQuery", map[string]any{
		"after":           after,
		"before":          nil,
		"first":           count,
		"last":            nil,
		"folder":          folder,
		"id":              c.threadsMailboxID,
		"use_micg_thread": false,
	})
}

func (c *Client) threadsWrapThreads(edges []slidetypes.Node[*slidetypes.ThreadsThread]) []slidetypes.Node[slidetypes.WrappedThreadInfo] {
	viewer := c.threadsViewer()
	out := make([]slidetypes.Node[slidetypes.WrappedThreadInfo], 0, len(edges))
	for _, edge := range edges {
		if edge.Node == nil {
			continue
		}
		out = append(out, slidetypes.Node[slidetypes.WrappedThreadInfo]{Node: edge.Node.ToWrapped(viewer), Cursor: edge.Cursor})
	}
	return out
}

func (c *Client) threadsGetMailbox(ctx context.Context) (*slidetypes.MailboxResponse, error) {
	setup, err := c.threadsMailboxSetup(ctx)
	if err != nil {
		return nil, err
	}
	list, err := c.threadsThreadList(ctx, "INBOX", "", threadsMailboxPageSize)
	if err != nil {
		return nil, err
	}
	return &slidetypes.MailboxResponse{Mailbox: &slidetypes.Mailbox{
		UQSeqID: setup.Mailbox.UQSeqID,
		ID:      setup.Mailbox.ID,
		ThreadsByFolder: slidetypes.Edged[slidetypes.Node[slidetypes.WrappedThreadInfo]]{
			Edges:    c.threadsWrapThreads(list.Mailbox.ThreadsByFolder.Edges),
			PageInfo: list.Mailbox.ThreadsByFolder.PageInfo,
		},
	}}, nil
}

func (c *Client) threadsPaginateMailbox(ctx context.Context, req *slidetypes.PaginateMailboxRequest) (*slidetypes.PaginateMailboxResponse, error) {
	list, err := c.threadsThreadList(ctx, req.Folder, req.Cursor, max(req.Count, 5))
	if err != nil {
		return nil, err
	}
	return &slidetypes.PaginateMailboxResponse{Mailbox: &slidetypes.MailboxPage{
		ID: list.Mailbox.ID,
		ThreadsByFolder: slidetypes.Edged[slidetypes.Node[slidetypes.WrappedThreadInfo]]{
			Edges:    c.threadsWrapThreads(list.Mailbox.ThreadsByFolder.Edges),
			PageInfo: list.Mailbox.ThreadsByFolder.PageInfo,
		},
	}}, nil
}

func (c *Client) threadsMessageList(ctx context.Context, threadID string, after *string, count int) (*slidetypes.ThreadsThread, error) {
	resp, err := threadsQuery[*slidetypes.ThreadsMessageListResponse](ctx, c, "BarcelonaMessageListPaginationQuery", map[string]any{
		"after":            after,
		"before":           nil,
		"first":            max(count, 1),
		"last":             nil,
		"has_live_chats":   false,
		"id":               threadID,
		"use_micg_message": false,
		"use_micg_thread":  false,
		"use_micg_user":    false,
	})
	if err != nil {
		return nil, err
	} else if resp == nil || resp.Thread == nil || resp.Thread.ID == "" {
		return nil, ErrThreadNotFound
	}
	return resp.Thread, nil
}

func (c *Client) threadsGetThread(ctx context.Context, req *slidetypes.GetThreadInfoRequest) (*slidetypes.ThreadInfoResponse, error) {
	thread, err := c.threadsMessageList(ctx, req.ThreadFBID, nil, threadsMessagePageSize)
	if errors.Is(err, ErrThreadNotFound) {
		// DM portals are keyed by the other user's FBID (the Instagram thread key convention),
		// so look up the one-to-one thread with that user instead.
		userFBID, _ := strconv.ParseInt(req.ThreadFBID, 10, 64)
		if userFBID == 0 {
			return nil, err
		}
		var dm *slidetypes.ThreadsThread
		if dm, err = c.threadsCreate1To1(ctx, userFBID); err == nil {
			thread, err = c.threadsMessageList(ctx, dm.ID, nil, threadsMessagePageSize)
		}
	}
	if err != nil {
		return nil, err
	}
	return &slidetypes.ThreadInfoResponse{ThreadInfo: thread.ToWrapped(c.threadsViewer())}, nil
}

func (c *Client) threadsPaginateMessages(ctx context.Context, req *slidetypes.PaginateMessagesRequest) (*slidetypes.PaginateMessagesResponse, error) {
	after := req.AfterCursor
	if after != nil && *after == "" {
		after = nil
	}
	thread, err := c.threadsMessageList(ctx, req.ThreadID, after, cmpOr(req.FirstN, threadsMessagePageSize))
	if err != nil {
		return nil, err
	}
	info := thread.ToThreadInfo(c.threadsViewer())
	return &slidetypes.PaginateMessagesResponse{ThreadInfo: slidetypes.AsThread[slidetypes.MessagesOnlyThread]{
		ID:               thread.ID,
		AsIGDirectThread: &slidetypes.MessagesOnlyThread{ID: thread.ID, Messages: info.SlideMessages},
	}}, nil
}

func cmpOr(vals ...int) int {
	for _, v := range vals {
		if v != 0 {
			return v
		}
	}
	return 0
}

func (c *Client) threadsSendMessage(ctx context.Context, req *slidetypes.SendTextRequest) (*slidetypes.SendTextResponse, error) {
	if (req.IGThreadIGID == nil || *req.IGThreadIGID == "") && len(req.RecipientIGIDs) == 1 {
		// A new DM: the connector passes the other user's FBID as the recipient.
		userFBID, _ := strconv.ParseInt(req.RecipientIGIDs[0], 10, 64)
		if userFBID != 0 {
			if dm, err := c.threadsCreate1To1(ctx, userFBID); err != nil {
				return nil, fmt.Errorf("failed to open DM thread: %w", err)
			} else {
				req.IGThreadIGID = &dm.ID
			}
		}
	}
	if req.IGThreadIGID == nil || *req.IGThreadIGID == "" {
		return nil, fmt.Errorf("thread ID missing from Threads send request")
	}
	resp, err := threadsQuery[*slidetypes.ThreadsSendTextResponse](ctx, c, "WmiSlideTextMessageMutation", map[string]any{
		"text_body":             req.Text,
		"recipient_fbids":       nil,
		"thread_fbid":           *req.IGThreadIGID,
		"offline_threading_id":  threadsOTID(req.OfflineThreadingID),
		"replied_to_message_id": req.ReplyToMessageID,
		"forwarded_message_id":  nil,
		"send_silently":         false,
		// TODO find the threads.com mention format; mentions are sent as plain text for now
		"mentioned_users": nil,
	})
	if err != nil {
		return nil, err
	}
	return &slidetypes.SendTextResponse{Message: resp.Message.ToSentMessage()}, nil
}

func (c *Client) threadsSendMedia(ctx context.Context, req *slidetypes.SendMediaRequest) (*slidetypes.SendMediaResponse, error) {
	attachmentType := req.ThreadsAttachmentType
	if attachmentType == "" {
		attachmentType = "IMAGE"
	}
	resp, err := threadsQuery[*slidetypes.ThreadsSendMediaResponse](ctx, c, "WmiSlideMediaMessageMutation", map[string]any{
		"offline_threading_id":  threadsOTID(req.OfflineThreadingID),
		"thread_fbid":           req.ThreadID,
		"media":                 map[string]any{"attachment_type": attachmentType, "media_fbid": req.AttachmentFBID},
		"replied_to_message_id": req.ReplyToMessageID,
	})
	if err != nil {
		return nil, err
	}
	return &slidetypes.SendMediaResponse{Message: resp.Message.ToSentMessage()}, nil
}

func (c *Client) threadsEditMessage(ctx context.Context, req *slidetypes.EditMessageRequest) (*slidetypes.EditMessageResponse, error) {
	messageID := req.TargetMessageID
	if messageID == "" {
		messageID = req.TargetItemID
	}
	_, err := threadsQuery[noResp](ctx, c, "WmiSlideEditMessageMutation", map[string]any{
		"message_id":           messageID,
		"new_text_body":        req.Body,
		"offline_threading_id": threadsOTID(req.OfflineThreadingID),
	})
	if err != nil {
		return nil, err
	}
	return &slidetypes.EditMessageResponse{Message: slidetypes.SentMessage{MessageID: messageID}}, nil
}

func (c *Client) threadsUnsendMessage(ctx context.Context, req *slidetypes.UnsendMessageRequest) (*slidetypes.UnsendMessageResponse, error) {
	resp, err := threadsQuery[map[string]any](ctx, c, "WmiSlideUnsendForEveryoneMessageMutation", map[string]any{
		"message_id":           req.MessageID,
		"offline_threading_id": threadsOTID(""),
	})
	if err != nil {
		return nil, err
	}
	ok, _ := resp["slide_unsend_message"].(bool)
	return &slidetypes.UnsendMessageResponse{DirectUnsendMessage: ok}, nil
}

func (c *Client) threadsSendReaction(ctx context.Context, req *slidetypes.CreateReactionRequest) (*slidetypes.SendReactionResponse, error) {
	vars := map[string]any{
		"message_id":           req.Input.MessageID,
		"offline_threading_id": threadsOTID(""),
	}
	name := "WmiSlideRemoveReactionMutation"
	if req.Input.ReactionStatus != slidetypes.ReactionStatusDeleted {
		name = "WmiSlideSetReactionMutation"
		vars["reaction"] = req.Input.Emoji
	}
	resp, err := threadsQuery[*slidetypes.ThreadsReactionResponse](ctx, c, name, vars)
	if err != nil {
		return nil, err
	}
	out := &slidetypes.SendReactionResponse{}
	msg := resp.SetReaction
	if msg == nil {
		msg = resp.RemoveReaction
	}
	if msg != nil {
		out.Message.ID = msg.ID
		out.Message.Reactions = msg.Interface.Reactions
	}
	return out, nil
}

// threadsMarkRead marks the thread read up to the given timestamp. threads.com takes a timestamp
// rather than a message ID, so it's sent from MarkReadValidation, which carries the timestamp.
func (c *Client) threadsMarkRead(ctx context.Context, req *slidetypes.MarkReadRequest) (*slidetypes.MarkReadValidationResponse, error) {
	ts := req.Data.MessageTimestampMS.Time
	if ts.IsZero() {
		ts = time.Now()
	}
	_, err := threadsQuery[noResp](ctx, c, "WmiSlideThreadMutationsMarkReadMutation", map[string]any{
		"thread_fbid":     req.Metadata.IGThreadIGID,
		"timestamp_ms":    strconv.FormatInt(ts.UnixMilli(), 10),
		"use_micg_thread": false,
	})
	if err != nil {
		return nil, err
	}
	return &slidetypes.MarkReadValidationResponse{Validated: true}, nil
}

func (c *Client) threadsSetTyping(ctx context.Context, threadID string, typing bool) error {
	status := 0
	if typing {
		status = 1
	}
	_, err := makeGraphQLRequest[noResp](ctx, c, "WmiSlidePublishTypingIndicatorMutation", map[string]any{
		"activity_status": status,
		"thread_id":       threadID,
		"ttl":             400,
	}, true)
	return err
}

func (c *Client) threadsSearchUsers(ctx context.Context, query string) (*slidetypes.SearchResponse, error) {
	resp, err := threadsQuery[*slidetypes.ThreadsSearchResponse](ctx, c, "BarcelonaNewThreadContactSearchResultsQuery", map[string]any{
		"query": query,
	})
	if err != nil {
		return nil, err
	}
	out := &slidetypes.SearchResponse{}
	for i := range resp.Search.Results {
		if user := resp.Search.Results[i].User(); user.ID != 0 {
			out.Data.Results = append(out.Data.Results, user.ToSearchResult())
		}
	}
	return out, nil
}

// threadsCreate1To1 opens (or creates) the one-to-one thread with a user. threads.com returns the
// existing thread if there is one.
func (c *Client) threadsCreate1To1(ctx context.Context, userFBID int64) (*slidetypes.ThreadsThread, error) {
	resp, err := threadsQuery[*slidetypes.ThreadsCreate1To1Response](ctx, c, "WmiSlideThreadMutationsCreate1To1Mutation", map[string]any{
		"recipient_fbid":       strconv.FormatInt(userFBID, 10),
		"offline_threading_id": threadsOTID(""),
	})
	if err != nil {
		return nil, err
	} else if resp == nil || resp.Thread == nil || resp.Thread.ID == "" {
		return nil, ErrThreadNotFound
	}
	return resp.Thread, nil
}

func (c *Client) threadsFetchThreadID(ctx context.Context, fbid int64) (*ThreadIGIDs, error) {
	// For groups the portal ID is already the thread FBID.
	if thread, err := c.threadsMessageList(ctx, strconv.FormatInt(fbid, 10), nil, 1); err == nil {
		return &ThreadIGIDs{ShortID: thread.ID, LongID: thread.ID}, nil
	}
	// For DMs it's the other user's FBID.
	thread, err := c.threadsCreate1To1(ctx, fbid)
	if err != nil {
		return nil, err
	}
	return &ThreadIGIDs{ShortID: thread.ID, LongID: thread.ID}, nil
}

func (c *Client) threadsGetUserForNewDM(ctx context.Context, fbid int64) (*slidetypes.UserInfoResponse, error) {
	thread, err := c.threadsCreate1To1(ctx, fbid)
	if err != nil {
		return nil, err
	}
	for _, member := range thread.Members.Nodes {
		if member != nil && member.ID == fbid {
			return &slidetypes.UserInfoResponse{Data: member.ToUser()}, nil
		}
	}
	return nil, fmt.Errorf("user %d not found in the new thread", fbid)
}

func (c *Client) threadsCreateGroup(ctx context.Context, req *slidetypes.CreateGroupRequest) (*slidetypes.CreateGroupResponse, error) {
	resp, err := threadsQuery[map[string]threadsIDOnly](ctx, c, "WmiSlideThreadMutationsCreateGroupMutation", map[string]any{
		"participant_fbids":    req.RecipientFBIDs,
		"thread_name":          nil,
		"offline_threading_id": threadsOTID(req.OfflineThreadingID),
	})
	if err != nil {
		return nil, err
	}
	out := &slidetypes.CreateGroupResponse{}
	for _, thread := range resp {
		if thread.ID != "" {
			out.Data.ID = thread.ID
			out.Data.Thread = &slidetypes.FolderOnlyThread{ThreadKey: thread.ID, ID: thread.ID, Folder: thread.Folder}
			break
		}
	}
	if out.Data.ID == "" {
		return nil, fmt.Errorf("group creation response didn't include a thread ID")
	}
	return out, nil
}

type threadsIDOnly struct {
	ID     string `json:"id"`
	Folder string `json:"folder"`
}

func (c *Client) threadsBoolMutation(ctx context.Context, name string, vars map[string]any) error {
	_, err := makeGraphQLRequest[noResp](ctx, c, name, vars, true)
	return err
}

func (c *Client) threadsAddMembers(ctx context.Context, req *slidetypes.AddMembersRequest) (*slidetypes.AddMembersResponse, error) {
	return &slidetypes.AddMembersResponse{}, c.threadsBoolMutation(ctx, "WmiSlideAddParticipantsMutation", map[string]any{
		"message_recipient_ids_to_add": req.ThreadsUserFBIDs,
		"offline_threading_id":         threadsOTID(""),
		"thread_fbid":                  req.ThreadID,
	})
}

func (c *Client) threadsRemoveMember(ctx context.Context, threadID, participantFBID string) error {
	return c.threadsBoolMutation(ctx, "WmiSlideRemoveParticipantMutation", map[string]any{
		"participant_fbid": participantFBID,
		"thread_fbid":      threadID,
	})
}

func (c *Client) threadsModifyAdmins(ctx context.Context, threadID string, fbids []string, add bool) error {
	name, key := "WmiSlideAdminManagementRemoveAdminsMutation", "admins_to_remove"
	if add {
		name, key = "WmiSlideAdminManagementAddAdminsMutation", "admins_to_add"
	}
	return c.threadsBoolMutation(ctx, name, map[string]any{
		key:                    fbids,
		"offline_threading_id": threadsOTID(""),
		"thread_fbid":          threadID,
	})
}

func (c *Client) threadsEditGroupTitle(ctx context.Context, threadID, title string) error {
	return c.threadsBoolMutation(ctx, "WmiSlideChangeThreadNameMutation", map[string]any{
		"offline_threading_id": threadsOTID(""),
		"thread_id":            threadID,
		"thread_name":          title,
	})
}

func (c *Client) threadsDeleteThread(ctx context.Context, threadID string) error {
	return c.threadsBoolMutation(ctx, "WmiSlideThreadMutationsDeleteMutation", map[string]any{
		"thread_fbid": threadID,
	})
}

func (c *Client) threadsMuteThread(ctx context.Context, threadID string, muteSeconds int) error {
	return c.threadsBoolMutation(ctx, "WmiSlideThreadMutationsMuteMutation", map[string]any{
		"mute_duration_seconds": muteSeconds,
		"offline_threading_id":  threadsOTID(""),
		"thread_id":             threadID,
	})
}

func (c *Client) threadsAcceptMessageRequest(ctx context.Context, threadID string) error {
	return c.threadsBoolMutation(ctx, "useBarcelonaMessagesThreadAcceptActionMutation", map[string]any{
		"thread_fbid": threadID,
	})
}
