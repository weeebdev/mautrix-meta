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
	"fmt"
	"strconv"

	"github.com/rs/zerolog"

	"go.mau.fi/mautrix-meta/pkg/instameow/slidetypes"
	"go.mau.fi/mautrix-meta/pkg/messagix/graphql"
	"go.mau.fi/mautrix-meta/pkg/messagix/types"
)

func (c *Client) SendMessage(ctx context.Context, req *slidetypes.SendTextRequest) (*slidetypes.SendTextResponse, error) {
	if c.IsThreads() {
		return c.threadsSendMessage(ctx, req)
	}
	return makeGraphQLRequest[*slidetypes.SendTextResponse](ctx, c, "IGDirectTextSendMutation", req, true)
}

func (c *Client) SendMedia(ctx context.Context, req *slidetypes.SendMediaRequest) (*slidetypes.SendMediaResponse, error) {
	if c.IsThreads() {
		return c.threadsSendMedia(ctx, req)
	}
	return makeGraphQLRequest[*slidetypes.SendMediaResponse](ctx, c, "IGDirectMediaSendMutation", req, true)
}

func (c *Client) EditMessage(ctx context.Context, req *slidetypes.EditMessageRequest) (*slidetypes.EditMessageResponse, error) {
	if c.IsThreads() {
		return c.threadsEditMessage(ctx, req)
	}
	return makeGraphQLRequest[*slidetypes.EditMessageResponse](ctx, c, "IGDirectEditMessageMutation", req, true)
}

func (c *Client) UnsendMessage(ctx context.Context, req *slidetypes.UnsendMessageRequest) (*slidetypes.UnsendMessageResponse, error) {
	if c.IsThreads() {
		return c.threadsUnsendMessage(ctx, req)
	}
	return makeGraphQLRequest[*slidetypes.UnsendMessageResponse](ctx, c, "IGDMessageUnsendDialogOffMsysMutation", req, true)
}

func (c *Client) SendReaction(ctx context.Context, req *slidetypes.CreateReactionRequest) (*slidetypes.SendReactionResponse, error) {
	if c.IsThreads() {
		return c.threadsSendReaction(ctx, req)
	}
	return makeGraphQLRequest[*slidetypes.SendReactionResponse](ctx, c, "IGDirectReactionSendMutation", req, true)
}

func (c *Client) MarkRead(ctx context.Context, req *slidetypes.MarkReadRequest) (*slidetypes.MarkReadResponse, error) {
	if c.IsThreads() {
		// Sent from MarkReadValidation, which has the timestamp threads.com needs
		return &slidetypes.MarkReadResponse{}, nil
	}
	return makeGraphQLRequest[*slidetypes.MarkReadResponse](ctx, c, "useIGDMarkThreadAsReadMutation", req, true)
}

func (c *Client) MarkReadValidation(ctx context.Context, req *slidetypes.MarkReadRequest) (*slidetypes.MarkReadValidationResponse, error) {
	if c.IsThreads() {
		return c.threadsMarkRead(ctx, req)
	}
	return makeGraphQLRequest[*slidetypes.MarkReadValidationResponse](ctx, c, "useIGDMarkThreadAsReadValidationMutation", req, false)
}

func (c *Client) DeleteThread(ctx context.Context, req *slidetypes.DeleteThreadRequest) (*slidetypes.DeleteThreadResponse, error) {
	if c.IsThreads() {
		return &slidetypes.DeleteThreadResponse{}, c.threadsDeleteThread(ctx, req.ThreadID)
	}
	return makeGraphQLRequest[*slidetypes.DeleteThreadResponse](ctx, c, "IGDInboxInfoDeleteThreadDialogOffMsysMutation", req, true)
}

func (c *Client) LeaveGroup(ctx context.Context, req *slidetypes.LeaveThreadRequest) (*slidetypes.LeaveThreadResponse, error) {
	if c.IsThreads() {
		return &slidetypes.LeaveThreadResponse{}, c.threadsRemoveMember(ctx, req.ThreadID, strconv.FormatInt(c.threadsOwnFBID(), 10))
	}
	return makeGraphQLRequest[*slidetypes.LeaveThreadResponse](ctx, c, "useIGDLeaveChatMutation", req, true)
}

func (c *Client) RemoveMember(ctx context.Context, req *slidetypes.RemoveMemberRequest) (*slidetypes.RemoveMemberResponse, error) {
	if c.IsThreads() {
		return &slidetypes.RemoveMemberResponse{}, c.threadsRemoveMember(ctx, req.ThreadID, req.ThreadsUserFBID)
	}
	return makeGraphQLRequest[*slidetypes.RemoveMemberResponse](ctx, c, "IGDRemoveFromGroupDialogItemOffMsysMutation", req, true)
}

func (c *Client) AddMembers(ctx context.Context, req *slidetypes.AddMembersRequest) (*slidetypes.AddMembersResponse, error) {
	if c.IsThreads() {
		return c.threadsAddMembers(ctx, req)
	}
	return makeGraphQLRequest[*slidetypes.AddMembersResponse](ctx, c, "IGDAddParticipantSubmitBarMutation", req, true)
}

func (c *Client) AddAdmins(ctx context.Context, req *slidetypes.ModifyAdminsRequest) (*slidetypes.AddAdminsResponse, error) {
	if c.IsThreads() {
		return &slidetypes.AddAdminsResponse{}, c.threadsModifyAdmins(ctx, req.ThreadID, req.ThreadsUserFBIDs, true)
	}
	return makeGraphQLRequest[*slidetypes.AddAdminsResponse](ctx, c, "IGDAddAdminDialogItemOffMsysMutation", req, true)
}

func (c *Client) RemoveAdmins(ctx context.Context, req *slidetypes.ModifyAdminsRequest) (*slidetypes.RemoveAdminsResponse, error) {
	if c.IsThreads() {
		return &slidetypes.RemoveAdminsResponse{}, c.threadsModifyAdmins(ctx, req.ThreadID, req.ThreadsUserFBIDs, false)
	}
	return makeGraphQLRequest[*slidetypes.RemoveAdminsResponse](ctx, c, "IGDRemoveAdminDialogItemOffMsysMutation", req, true)
}

func (c *Client) AcceptMessageRequest(ctx context.Context, req *slidetypes.AcceptMessageRequestRequest) (*slidetypes.AcceptMessageRequestResponse, error) {
	if c.IsThreads() {
		return &slidetypes.AcceptMessageRequestResponse{}, c.threadsAcceptMessageRequest(ctx, req.ThreadID)
	}
	return makeGraphQLRequest[*slidetypes.AcceptMessageRequestResponse](ctx, c, "useIGDirectAcceptMessageRequestMutation", req, true)
}

func (c *Client) MuteThread(ctx context.Context, req *slidetypes.MuteThreadRequest) (*slidetypes.MuteThreadResponse, error) {
	if c.IsThreads() {
		return &slidetypes.MuteThreadResponse{}, c.threadsMuteThread(ctx, req.ThreadID, req.MuteSeconds)
	}
	return makeGraphQLRequest[*slidetypes.MuteThreadResponse](ctx, c, "IGDInboxInfoMuteToggleOffMsysMutation", req, true)
}

func (c *Client) PinThread(ctx context.Context, req *slidetypes.PinThreadRequest) (*slidetypes.PinThreadResponse, error) {
	if c.IsThreads() {
		return nil, ErrNotSupportedOnThreads
	}
	return makeGraphQLRequest[*slidetypes.PinThreadResponse](ctx, c, "useIGDPinThreadMutation", req, true)
}

func (c *Client) PinMessage(ctx context.Context, req *slidetypes.PinMessageRequest) (*slidetypes.PinMessageResponse, error) {
	if c.IsThreads() {
		return nil, ErrNotSupportedOnThreads
	}
	return makeGraphQLRequest[*slidetypes.PinMessageResponse](ctx, c, "useIGDPinMessageOffMsysMutation", req, true)
}

func (c *Client) UnpinMessage(ctx context.Context, req *slidetypes.PinMessageRequest) (*slidetypes.UnpinMessageResponse, error) {
	if c.IsThreads() {
		return nil, ErrNotSupportedOnThreads
	}
	return makeGraphQLRequest[*slidetypes.UnpinMessageResponse](ctx, c, "useIGDUnpinMessageOffMsysMutation", req, true)
}

func (c *Client) GetMailbox(ctx context.Context) (*slidetypes.MailboxResponse, error) {
	if c.IsThreads() {
		return c.threadsGetMailbox(ctx)
	}
	return makeGraphQLRequest[*slidetypes.MailboxResponse](
		ctx, c, "PolarisDirectInboxQuery",
		slidetypes.MakeMailboxRequest(c.configs.BrowserConfigTable.IGDMqttWebDeviceID.ClientID),
		false,
	)
}

func (c *Client) PaginateMailbox(ctx context.Context, req *slidetypes.PaginateMailboxRequest) (*slidetypes.PaginateMailboxResponse, error) {
	if c.IsThreads() {
		return c.threadsPaginateMailbox(ctx, req)
	}
	return makeGraphQLRequest[*slidetypes.PaginateMailboxResponse](ctx, c, "IGDThreadListOffMsysPaginationQuery", req, true)
}

func (c *Client) GetThread(ctx context.Context, req *slidetypes.GetThreadInfoRequest) (*slidetypes.ThreadInfoResponse, error) {
	if c.IsThreads() {
		return c.threadsGetThread(ctx, req)
	}
	return makeGraphQLRequest[*slidetypes.ThreadInfoResponse](ctx, c, "IGDThreadDetailQuery", req, true)
}

func (c *Client) PaginateMessages(ctx context.Context, req *slidetypes.PaginateMessagesRequest) (*slidetypes.PaginateMessagesResponse, error) {
	if c.IsThreads() {
		return c.threadsPaginateMessages(ctx, req)
	}
	return makeGraphQLRequest[*slidetypes.PaginateMessagesResponse](ctx, c, "IGDMessageListOffMsysQuery", req, true)
}

func (c *Client) GetProfile(ctx context.Context, igid string) (*slidetypes.ProfilePageResponse, error) {
	if c.IsThreads() {
		return nil, ErrNotSupportedOnThreads
	}
	return makeGraphQLRequest[*slidetypes.ProfilePageResponse](ctx, c, "PolarisProfilePageContentQuery", slidetypes.MakeProfilePageRequest(igid), true)
}

func (c *Client) GetUserForNewDM(ctx context.Context, fbid int64) (*slidetypes.UserInfoResponse, error) {
	if c.IsThreads() {
		return c.threadsGetUserForNewDM(ctx, fbid)
	}
	return makeGraphQLRequest[*slidetypes.UserInfoResponse](ctx, c, "useIGDCreateOptimisticThreadUserQuery", &slidetypes.GetUserInfoByFBIDRequest{MessagingUserFBID: fbid}, true)
}

func (c *Client) SearchUsers(ctx context.Context, query string) (*slidetypes.SearchResponse, error) {
	if c.IsThreads() {
		return c.threadsSearchUsers(ctx, query)
	}
	return makeGraphQLRequest[*slidetypes.SearchResponse](ctx, c, "IGDOmniPickerSearchResultsListQuery", slidetypes.SearchRequest{SearchText: query}, true)
}

func (c *Client) CreateGroup(ctx context.Context, req *slidetypes.CreateGroupRequest) (*slidetypes.CreateGroupResponse, error) {
	if c.IsThreads() {
		return c.threadsCreateGroup(ctx, req)
	}
	return makeGraphQLRequest[*slidetypes.CreateGroupResponse](ctx, c, "useCreateOpenGroupThreadOffMsysMutation", req, true)
}

func (c *Client) EditGroupTitle(ctx context.Context, threadID, newTitle string) error {
	if c.IsThreads() {
		return c.threadsEditGroupTitle(ctx, threadID, newTitle)
	}
	_, err := makeGraphQLRequest[noResp](ctx, c, "IGDEditThreadNameDialogOffMsysMutation", &graphql.IGEditGroupTitleGraphQLRequestPayload{
		ThreadID: threadID,
		NewTitle: newTitle,
	}, true)
	return err
}

func (c *Client) SetUserBlocked(ctx context.Context, userID string, blocked bool) error {
	if c.IsThreads() {
		return ErrNotSupportedOnThreads
	}
	name := "IGDBlockUserDialogOffMsysUnblockMutation"
	variables := map[string]any{"target_user_id": userID}
	if blocked {
		name = "IGDBlockUserDialogOffMsysMutation"
		variables["surface"] = "direct_thread_info"
	}
	resp, err := makeGraphQLRequest[map[string]any](ctx, c, name, variables, true)
	if err != nil {
		return err
	}
	field := "xdt_unblock"
	if blocked {
		field = "xdt_block_v2"
	}
	if resp[field] == nil {
		return fmt.Errorf("missing %s in block status response", field)
	}
	return nil
}

type noResp = json.RawMessage

func makeGraphQLRequest[T any](ctx context.Context, c *Client, name string, req any, allowReload bool) (resp T, err error) {
	if c == nil {
		err = ErrClientIsNil
		return
	}
	_, respData, err := c.http.MakeGraphQLRequest(ctx, name, req)
	if err != nil {
		c.checkResponseError(err)
		return
	}
	var wrappedResp graphql.Response[T]
	err = json.Unmarshal(respData, &wrappedResp)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal %T: %w", resp, err)
		return
	} else if allowReload && wrappedResp.ErrorCode == types.ErrPleaseReloadPage.ErrorCode {
		zerolog.Ctx(ctx).Warn().Err(wrappedResp.AsError()).
			Msg("Got please reload page error, reloading index and retrying")
		didReload, reloadErr := c.ReloadIndex(ctx)
		if reloadErr != nil {
			zerolog.Ctx(ctx).Err(err).Msg("Failed to reload page to retry GraphQL request")
		} else if didReload {
			zerolog.Ctx(ctx).Debug().Msg("Successfully reloaded index, retrying GraphQL request")
			return makeGraphQLRequest[T](ctx, c, name, req, false)
		} else {
			zerolog.Ctx(ctx).Debug().Msg("Didn't reload index, not retrying GraphQL request")
		}
	}
	return wrappedResp.Data, wrappedResp.AsError()
}
