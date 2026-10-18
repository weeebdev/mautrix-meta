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
	"testing"
)

// The fixtures below follow the shape of threads.com responses with placeholder values.

const testViewerFBID = 90000000000000001

func testViewer() *User {
	return &User{InteropMessagingUserFBID: testViewerFBID, ID: "40000000001", Username: "viewer"}
}

const testMessagesJSON = `{
	"fetch__SlideThread": {
		"id": "1000000000000001",
		"folder": "INBOX",
		"thread_name": "",
		"is_group": false,
		"thread_type": "ONE_TO_ONE",
		"admin_fbids": [],
		"group_image_emoji": "x",
		"group_image_gradient": ["FF9AD7FF", "FF6BBBEF"],
		"members": {"count": 2, "nodes": [
			{"id": "90000000000000002", "igid": "40000000002", "username": "friend", "name": "Friend", "profile_pic_uri": "https://example.invalid/a.jpg", "is_verified": false, "blocking": false, "following": true, "restricting": false},
			{"id": "90000000000000001", "igid": "40000000001", "username": "viewer", "name": "Viewer"}
		]},
		"read_receipts": [{"participant_fbid": "90000000000000002", "watermark_timestamp_ms": "1760800000000"}],
		"messages": {
			"edges": [
				{"node": {
					"message_id": "mid.$placeholder1", "sender_fbid": "90000000000000002", "timestamp_ms": "1760800000000",
					"reactions": [{"__typename": "SlideReaction", "reaction": "❤️", "sender_fbid": "90000000000000001", "reaction_timestamp_ms": "1760800001000"}],
					"id": "mid.$placeholder1", "__typename": "SlideMessage", "thread_fbid": "1000000000000001",
					"offline_threading_id": "7000000000000000001", "is_forwarded": true, "msg_edit_history": [],
					"mentioned_users_without_id": [], "replied_to_message": null,
					"content": {"__typename": "SlideMessageText", "__isSlideMessageContent": "SlideMessageText", "text_body": "hello"}
				}, "cursor": "c1"},
				{"node": {
					"message_id": "mid.$placeholder2", "sender_fbid": "90000000000000001", "timestamp_ms": "1760800002000",
					"reactions": [], "id": "mid.$placeholder2", "__typename": "SlideMessage", "thread_fbid": "1000000000000001",
					"offline_threading_id": "7000000000000000002", "is_forwarded": false, "msg_edit_history": [],
					"content": {"__typename": "SlideMessageImageContent", "attachments": [{"__typename": "SlideMessageImage", "attachment_fbid": "3000000000000001", "attachment_cdn_url": "https://example.invalid/i.jpg", "attachment_type": 1, "width": 64, "height": 64, "preview_cdn_url": "https://example.invalid/p.jpg", "preview_width": 64, "preview_height": 64}]}
				}, "cursor": "c2"},
				{"node": {
					"message_id": "mid.$placeholder3", "sender_fbid": "90000000000000002", "timestamp_ms": "1760800003000",
					"reactions": [], "id": "mid.$placeholder3", "__typename": "SlideMessage", "thread_fbid": "1000000000000001",
					"content": {"__typename": "SlideMessageThreadsPostShare", "text_post_app_share_context": "x", "threads_post_share_text_body": "look", "threads_post_text_body": "post text", "threads_post_json": "{\"code\":\"ABC123\",\"user\":{\"username\":\"author\"}}", "content_ref": {"media_igid": "1", "owner_igid": "2", "compound_media_id": "1_2"}}
				}, "cursor": "c3"}
			],
			"page_info": {"end_cursor": "c3", "has_next_page": true, "has_previous_page": false, "start_cursor": "c1"}
		}
	}
}`

func TestThreadsDMToThreadInfo(t *testing.T) {
	var resp ThreadsMessageListResponse
	if err := json.Unmarshal([]byte(testMessagesJSON), &resp); err != nil {
		t.Fatal(err)
	}
	info := resp.Thread.ToThreadInfo(testViewer())
	if info.ID != "1000000000000001" || info.ThreadFBID != info.ID || info.ThreadID != info.ID {
		t.Errorf("thread IDs not set from thread_fbid: %+v", info)
	}
	if len(info.Users) != 1 || info.Users[0].InteropMessagingUserFBID != 90000000000000002 || info.Users[0].ID != "40000000002" {
		t.Fatalf("viewer not filtered out of users or wrong user mapping: %+v", info.Users)
	}
	if info.ThreadKey != 90000000000000002 {
		t.Errorf("DM thread key should be the other user's FBID, got %d", info.ThreadKey)
	}
	if info.ThreadSubtype != ThreadSubtypeDirect || info.IsGroup {
		t.Errorf("unexpected DM subtype %q", info.ThreadSubtype)
	}
	if len(info.SlideReadReceipts) != 1 || info.SlideReadReceipts[0].ParticipantFBID != 90000000000000002 {
		t.Errorf("read receipts not mapped: %+v", info.SlideReadReceipts)
	}
	msgs := info.SlideMessages
	if msgs == nil || len(msgs.Edges) != 3 || msgs.PageInfo.EndCursor != "c3" || !msgs.PageInfo.HasNextPage {
		t.Fatalf("messages not mapped: %+v", msgs)
	}
	first := msgs.Edges[0].Node
	if text, ok := first.Content.Content.(*MessageContentText); !ok || text.TextBody != "hello" {
		t.Errorf("text content not decoded: %#v", first.Content.Content)
	}
	if !first.IGDIsForwarded {
		t.Error("is_forwarded not copied to IGDIsForwarded")
	}
	if first.SenderFBID != 90000000000000002 || len(first.Reactions) != 1 || first.Reactions[0].SenderFBID != testViewerFBID {
		t.Errorf("sender or reactions not decoded: %+v", first)
	}
	if img, ok := msgs.Edges[1].Node.Content.Content.(*MessageContentImage); !ok || len(img.Attachments) != 1 || img.Attachments[0].AttachmentFBID != "3000000000000001" {
		t.Errorf("image content not decoded: %#v", msgs.Edges[1].Node.Content.Content)
	}
	share, ok := msgs.Edges[2].Node.Content.Content.(*MessageContentThreadsPostShare)
	if !ok || share.PostTextBody != "post text" || share.ShareTextBody != "look" {
		t.Fatalf("post share not decoded: %#v", msgs.Edges[2].Node.Content.Content)
	}
	if got := share.PostURL(); got != "https://www.threads.com/@author/post/ABC123" {
		t.Errorf("unexpected post URL %q", got)
	}
}

func TestThreadsGroupToThreadInfo(t *testing.T) {
	thread := &ThreadsThread{
		ID:         "1000000000000002",
		IsGroup:    true,
		Folder:     "INBOX",
		ThreadName: "group",
		AdminFBIDs: []string{"90000000000000001", "90000000000000003"},
		ThreadImage: &struct {
			URI string `json:"uri"`
		}{URI: "https://example.invalid/g.jpg"},
		Members: ThreadsMemberList{Count: 3, Nodes: []*ThreadsUser{
			{ID: 90000000000000002, IGID: "40000000002"},
			{ID: 90000000000000003, IGID: "40000000003"},
			{ID: testViewerFBID, IGID: "40000000001"},
		}},
		MuteUntilMS: "-1",
	}
	info := thread.ToThreadInfo(testViewer())
	if info.ThreadKey != 1000000000000002 {
		t.Errorf("group thread key should be the thread FBID, got %d", info.ThreadKey)
	}
	if len(info.Users) != 2 {
		t.Errorf("expected 2 users without the viewer, got %d", len(info.Users))
	}
	if len(info.AdminUserIDs) != 2 || info.AdminUserIDs[0] != "40000000003" || info.AdminUserIDs[1] != "40000000001" {
		t.Errorf("admin FBIDs not mapped to IGIDs: %v", info.AdminUserIDs)
	}
	if info.ThreadImageURL != "https://example.invalid/g.jpg" || info.ThreadTitle != "group" {
		t.Errorf("group metadata not mapped: %+v", info)
	}
	if info.IsMuted == nil || !*info.IsMuted {
		t.Error("mute_until_ms -1 should mean muted")
	}
	if info.ThreadSubtype != ThreadSubtypeGroup {
		t.Errorf("unexpected group subtype %q", info.ThreadSubtype)
	}
}

func TestThreadsNoteToSelfThreadKey(t *testing.T) {
	thread := &ThreadsThread{
		ID:               "1000000000000003",
		ParticipantFBIDs: []string{"90000000000000001"},
		Members:          ThreadsMemberList{Count: 1, Nodes: []*ThreadsUser{{ID: testViewerFBID}}},
	}
	info := thread.ToThreadInfo(testViewer())
	if info.ThreadKey != testViewerFBID || len(info.Users) != 0 {
		t.Errorf("note to self should use the viewer FBID as thread key, got %d with %d users", info.ThreadKey, len(info.Users))
	}
}

func TestThreadsFolderMapping(t *testing.T) {
	for in, want := range map[string]string{"INBOX": "INBOX", "PENDING": "PENDING", "REQUESTS": "PENDING", "SPAM": "SPAM"} {
		if got := ThreadsFolderToSystemFolder(in); got != want {
			t.Errorf("folder %q mapped to %q, want %q", in, got, want)
		}
	}
}

func TestParseDeltaWrappers(t *testing.T) {
	instagram := `[{"data":{"slide_delta_processor":[{"__typename":"SlideUQPPMarkRead","uq_seq_id":"5","thread_fbid":"1"}]}}]`
	threads := `[{"data":{"slide_uqpp_deltas":[{"__typename":"SlideUQPPMarkRead","uq_seq_id":"6","thread_fbid":"2"}],"other":{"x":1}}}]`
	for name, raw := range map[string]string{"instagram": instagram, "threads": threads} {
		wrappers, err := ParseDeltaWrappers([]byte(raw))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(wrappers) != 1 || len(wrappers[0].Data.SlideDeltaProcessor) != 1 {
			t.Fatalf("%s: deltas not found: %+v", name, wrappers)
		}
		if d := wrappers[0].Data.SlideDeltaProcessor[0]; d.UQSeqID == 0 || d.TypeName != "SlideUQPPMarkRead" {
			t.Errorf("%s: delta not decoded: %+v", name, d)
		}
	}
}

func TestThreadsSearchResult(t *testing.T) {
	raw := `{"get_slide_search_results": {"rank_token": "x", "search_results": [
		{"__typename": "SlideUser", "id": "90000000000000002", "as_micg_interface": {"id": "90000000000000002", "username": "wrapped", "name": "W", "igid": "40000000002"}},
		{"__typename": "SlideUser", "id": "90000000000000003", "username": "flat", "name": "F", "igid": "40000000003"}
	]}}`
	var resp ThreadsSearchResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Search.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(resp.Search.Results))
	}
	if u := resp.Search.Results[0].User(); u.Username != "wrapped" || u.ID != 90000000000000002 {
		t.Errorf("wrapped result not used: %+v", u)
	}
	if r := resp.Search.Results[1].User().ToSearchResult(); r.Username != "flat" || r.InteropMessagingUserFBID != 90000000000000003 || r.PK != "40000000003" {
		t.Errorf("flat result not mapped: %+v", r)
	}
}
