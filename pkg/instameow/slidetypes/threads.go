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

package slidetypes

import (
	"encoding/json"
	"slices"
	"strconv"
	"time"

	"go.mau.fi/util/jsontime"
)

// threads.com returns the Slide message model (with its "micg" relay providers turned off), but
// threads and users have their own shape. These types decode that shape and convert it into the
// Instagram types, so the connector can treat both platforms the same way.

// ThreadsUser is a member, sender or search result on threads.com. ID is the messaging FBID (the
// same ID space as sender_fbid and admin_fbids); IGID is the Threads/Instagram user ID.
type ThreadsUser struct {
	ID                 int64  `json:"id,string"`
	IGID               string `json:"igid"`
	Username           string `json:"username"`
	Name               string `json:"name"`
	ProfilePicURI      string `json:"profile_pic_uri"`
	IsVerified         bool   `json:"is_verified"`
	Blocking           *bool  `json:"blocking"`
	Following          bool   `json:"following"`
	Restricting        bool   `json:"restricting"`
	ReachabilityStatus string `json:"reachability_status"`
}

func (u *ThreadsUser) ToUser() *User {
	user := &User{
		InteropMessagingUserFBID: u.ID,
		Username:                 u.Username,
		FullName:                 u.Name,
		ID:                       u.IGID,
		PK:                       u.IGID,
		ProfilePicURL:            u.ProfilePicURI,
		IsVerified:               u.IsVerified,
	}
	user.FriendshipStatus.Blocking = u.Blocking
	user.FriendshipStatus.Following = u.Following
	user.FriendshipStatus.IsRestricted = u.Restricting
	return user
}

func (u *ThreadsUser) ToSearchResult() *SearchResult {
	return &SearchResult{
		InteropMessagingUserFBID: u.ID,
		Username:                 u.Username,
		FullName:                 u.Name,
		PK:                       u.IGID,
		ProfilePicURL:            u.ProfilePicURI,
		IsVerified:               u.IsVerified,
	}
}

// ThreadsMessage is a Slide message from threads.com. A few fields are named differently than on
// Instagram; UnmarshalJSON copies them into the Instagram names.
type ThreadsMessage struct {
	Message
}

func (tm *ThreadsMessage) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &tm.Message); err != nil {
		return err
	}
	var extra struct {
		IsForwarded    bool                      `json:"is_forwarded"`
		MsgEditHistory []MessageEditHistoryEntry `json:"msg_edit_history"`
	}
	if err := json.Unmarshal(data, &extra); err != nil {
		return err
	}
	tm.IGDIsForwarded = tm.IGDIsForwarded || extra.IsForwarded
	if len(tm.SlideEditHistory) == 0 {
		tm.SlideEditHistory = extra.MsgEditHistory
	}
	return nil
}

type ThreadsMessages = Edged[Node[*ThreadsMessage]]

func threadsToSlideMessages(tm *ThreadsMessages) *SlideMessages {
	if tm == nil {
		return nil
	}
	out := &SlideMessages{PageInfo: tm.PageInfo, Edges: make([]Node[*Message], 0, len(tm.Edges))}
	for _, edge := range tm.Edges {
		if edge.Node == nil {
			continue
		}
		out.Edges = append(out.Edges, Node[*Message]{Node: &edge.Node.Message, Cursor: edge.Cursor})
	}
	return out
}

// ThreadsThread is a thread from the threads.com inbox list or message list queries.
type ThreadsThread struct {
	ID               string   `json:"id"`
	IsGroup          bool     `json:"is_group"`
	Folder           string   `json:"folder"`
	ThreadName       string   `json:"thread_name"`
	ThreadType       string   `json:"thread_type"`
	AdminFBIDs       []string `json:"admin_fbids"`
	ParticipantFBIDs []string `json:"participant_fbids"`
	ThreadImage      *struct {
		URI string `json:"uri"`
	} `json:"thread_image"`
	GroupImageEmoji    string                   `json:"group_image_emoji"`
	GroupImageGradient json.RawMessage          `json:"group_image_gradient"` // array of colors
	Members            ThreadsMemberList        `json:"members"`
	ReadReceipts       []ReadReceipt            `json:"read_receipts"`
	TimestampMS        jsontime.UnixMilliString `json:"timestamp_ms"`
	MuteUntilMS        string                   `json:"mute_until_ms"`
	IsUnread           bool                     `json:"is_unread"`
	Messages           *ThreadsMessages         `json:"messages"`

	LatestSnippetActivityContext *struct {
		TimestampMS jsontime.UnixMilliString `json:"timestamp_ms"`
	} `json:"latest_snippet_activity_context"`
}

type ThreadsMemberList struct {
	Count int            `json:"count"`
	Nodes []*ThreadsUser `json:"nodes"`
}

// ThreadsFolderToSystemFolder maps threads.com folder names to the Instagram system folders the
// connector checks for message requests.
func ThreadsFolderToSystemFolder(folder string) string {
	switch folder {
	case "PENDING", "MESSAGE_REQUESTS", "REQUESTS":
		return "PENDING"
	case "SPAM", "HIDDEN_REQUESTS":
		return "SPAM"
	default:
		return folder
	}
}

// ToThreadInfo converts the thread into Instagram's ThreadInfo. viewer is the logged-in user. The
// thread key follows the Instagram convention: the thread FBID for groups, and the other
// participant's FBID for one-to-one chats.
func (tt *ThreadsThread) ToThreadInfo(viewer *User) *ThreadInfo {
	threadFBID, _ := strconv.ParseInt(tt.ID, 10, 64)
	info := &ThreadInfo{
		ID:                      tt.ID,
		ThreadKey:               threadFBID,
		ThreadFBID:              tt.ID,
		ThreadID:                tt.ID,
		IsGroup:                 tt.IsGroup,
		Viewer:                  viewer,
		ThreadTitle:             tt.ThreadName,
		SlideReadReceipts:       tt.ReadReceipts,
		LastActivityTimestampMS: tt.TimestampMS,
		SystemFolder:            ThreadsFolderToSystemFolder(tt.Folder),
		Folder:                  tt.Folder,
		SlideMessages:           threadsToSlideMessages(tt.Messages),
	}
	if viewer != nil {
		info.ViewerID = viewer.ID
	}
	if tt.IsGroup {
		info.ThreadSubtype = ThreadSubtypeGroup
	} else {
		info.ThreadSubtype = ThreadSubtypeDirect
	}
	if tt.ThreadImage != nil {
		info.ThreadImageURL = tt.ThreadImage.URI
	}
	if tt.MuteUntilMS != "" {
		muteUntil, _ := strconv.ParseInt(tt.MuteUntilMS, 10, 64)
		info.IsMuted = new(muteUntil != 0 && (muteUntil < 0 || time.UnixMilli(muteUntil).After(time.Now())))
	}
	var viewerFBID int64
	if viewer != nil {
		viewerFBID = viewer.InteropMessagingUserFBID
	}
	adminIGIDs := make([]string, 0, len(tt.AdminFBIDs))
	for _, member := range tt.Members.Nodes {
		if member == nil {
			continue
		}
		if slices.Contains(tt.AdminFBIDs, strconv.FormatInt(member.ID, 10)) {
			adminIGIDs = append(adminIGIDs, member.IGID)
		}
		if member.ID == viewerFBID {
			continue
		}
		info.Users = append(info.Users, member.ToUser())
	}
	if viewer != nil && slices.Contains(tt.AdminFBIDs, strconv.FormatInt(viewerFBID, 10)) && !slices.Contains(adminIGIDs, viewer.ID) {
		adminIGIDs = append(adminIGIDs, viewer.ID)
	}
	if tt.IsGroup {
		info.AdminUserIDs = adminIGIDs
	} else if len(info.Users) == 1 {
		info.ThreadKey = info.Users[0].InteropMessagingUserFBID
	} else if len(info.Users) == 0 {
		for _, fbid := range tt.ParticipantFBIDs {
			if parsed, _ := strconv.ParseInt(fbid, 10, 64); parsed != 0 && parsed != viewerFBID {
				info.ThreadKey = parsed
				break
			}
		}
		if info.ThreadKey == threadFBID && viewerFBID != 0 {
			// Note to self: Instagram uses the viewer's own FBID as the thread key.
			info.ThreadKey = viewerFBID
		}
	}
	return info
}

func (tt *ThreadsThread) ToWrapped(viewer *User) WrappedThreadInfo {
	return WrappedThreadInfo{AsIGDirectThread: tt.ToThreadInfo(viewer), ID: tt.ID}
}

// ThreadsMailboxSetup is the response of BarcelonaMessagesSetup_MailboxQuery.
type ThreadsMailboxSetup struct {
	Mailbox struct {
		ID      string `json:"id"`
		UQSeqID int64  `json:"iris_inactive_subscription_uq_seq_id,string"`
	} `json:"get_slide_mailbox_for_iris_subscription"`
}

// ThreadsThreadListResponse is the response of BarcelonaMessagesThreadListPaginationQuery.
type ThreadsThreadListResponse struct {
	Mailbox struct {
		ID              string                      `json:"id"`
		ThreadsByFolder Edged[Node[*ThreadsThread]] `json:"threads_by_folder"`
	} `json:"fetch__SlideMailbox"`
}

// ThreadsMessageListResponse is the response of BarcelonaMessageListPaginationQuery, which returns
// the thread info together with a page of messages.
type ThreadsMessageListResponse struct {
	Thread *ThreadsThread `json:"fetch__SlideThread"`
}

// ThreadsSentMessage is the message returned by the threads.com send mutations.
type ThreadsSentMessage struct {
	MessageID   string                   `json:"message_id"`
	TimestampMS jsontime.UnixMilliString `json:"timestamp_ms"`
	ID          string                   `json:"id"`
}

func (tsm *ThreadsSentMessage) ToSentMessage() SentMessage {
	return SentMessage{MessageID: tsm.MessageID, TimestampMS: tsm.TimestampMS, ID: tsm.ID}
}

type ThreadsSendTextResponse struct {
	Message ThreadsSentMessage `json:"send_slide_text_message"`
}

type ThreadsSendMediaResponse struct {
	Message ThreadsSentMessage `json:"send_slide_media_message"`
}

type ThreadsReactionResponse struct {
	SetReaction    *ThreadsReactionMessage `json:"slide_set_reaction"`
	RemoveReaction *ThreadsReactionMessage `json:"slide_remove_reaction"`
}

type ThreadsReactionMessage struct {
	MessageID string `json:"message_id"`
	ID        string `json:"id"`
	Interface struct {
		Reactions []Reaction `json:"msg_reactions"`
	} `json:"as_micg_interface"`
}

type ThreadsCreate1To1Response struct {
	Thread *ThreadsThread `json:"create_slide_1to1_thread"`
}

type ThreadsSearchResponse struct {
	Search struct {
		Results []ThreadsSearchResult `json:"search_results"`
	} `json:"get_slide_search_results"`
}

// ThreadsSearchResult is either a flat user or one wrapped in as_micg_interface, depending on the
// relay providers.
type ThreadsSearchResult struct {
	ThreadsUser
	Interface *ThreadsUser `json:"as_micg_interface"`
}

func (tsr *ThreadsSearchResult) User() *ThreadsUser {
	if tsr.Interface != nil && tsr.Interface.ID != 0 {
		return tsr.Interface
	}
	return &tsr.ThreadsUser
}

// MessageContentThreadsPostShare is a shared Threads post (SlideMessageThreadsPostShare).
type MessageContentThreadsPostShare struct {
	ShareContext  string `json:"text_post_app_share_context"`
	ShareTextBody string `json:"threads_post_share_text_body"`
	PostTextBody  string `json:"threads_post_text_body"`
	PostJSON      string `json:"threads_post_json"`
	ContentRef    struct {
		MediaIGID       string `json:"media_igid"`
		OwnerIGID       string `json:"owner_igid"`
		CompoundMediaID string `json:"compound_media_id"`
	} `json:"content_ref"`

	Unrecognized map[string]any `json:",unknown"`
}

func (*MessageContentThreadsPostShare) isMessageContent() {}

// PostURL returns the threads.com URL of the shared post, if the embedded post JSON has the code.
func (c *MessageContentThreadsPostShare) PostURL() string {
	if c.PostJSON == "" {
		return ""
	}
	var post struct {
		Code string `json:"code"`
		User struct {
			Username string `json:"username"`
		} `json:"user"`
		Post *struct {
			Code string `json:"code"`
			User struct {
				Username string `json:"username"`
			} `json:"user"`
		} `json:"post"`
	}
	if json.Unmarshal([]byte(c.PostJSON), &post) != nil {
		return ""
	}
	code, username := post.Code, post.User.Username
	if code == "" && post.Post != nil {
		code, username = post.Post.Code, post.Post.User.Username
	}
	if code == "" {
		return ""
	}
	if username == "" {
		return "https://www.threads.com/t/" + code
	}
	return "https://www.threads.com/@" + username + "/post/" + code
}
