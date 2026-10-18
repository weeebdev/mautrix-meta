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
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	"maunium.net/go/mautrix/event"

	"go.mau.fi/mautrix-meta/pkg/metaid"
)

func testKey(b byte) string {
	return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{b}, 32))
}

func TestSealRoundTrip(t *testing.T) {
	s, err := newContentSealer(testKey(1))
	if err != nil {
		t.Fatal(err)
	}
	in := &storedContent{Content: &event.MessageEventContent{MsgType: event.MsgText, Body: "secret"}, EditHistory: []string{"older"}}
	sealed, err := s.Seal("mid.$x/", in)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sealed, "secret") {
		t.Fatal("sealed value contains plaintext")
	}
	out, err := s.Open("mid.$x/", sealed)
	if err != nil {
		t.Fatal(err)
	}
	if out.Content.Body != "secret" || len(out.EditHistory) != 1 || out.EditHistory[0] != "older" {
		t.Errorf("round trip mismatch: %+v", out)
	}
	if _, err = s.Open("mid.$other/", sealed); err == nil {
		t.Error("content opened with a different message ID")
	}
	other, _ := newContentSealer(testKey(2))
	if _, err = other.Open("mid.$x/", sealed); err == nil {
		t.Error("content opened with a different key")
	}
}

func TestSealerKeyValidation(t *testing.T) {
	if _, err := newContentSealer("not base64!"); err == nil {
		t.Error("invalid base64 accepted")
	}
	if _, err := newContentSealer(base64.StdEncoding.EncodeToString([]byte("short"))); err == nil {
		t.Error("short key accepted")
	}
	t.Setenv(contentKeyEnv, "")
	if s, err := newContentSealerFromEnv(); s != nil || err != nil {
		t.Error("expected no sealer without a key")
	}
}

func TestWithEditHistory(t *testing.T) {
	out := withEditHistory(&event.MessageEventContent{MsgType: event.MsgText, Body: "new <b>"}, []string{"old 2", "old\n1"})
	if out.Body != "new <b>\n✏️ was: old 2\n✏️ was: old\n1" {
		t.Errorf("unexpected body %q", out.Body)
	}
	if !strings.Contains(out.FormattedBody, "new &lt;b&gt;") || !strings.Contains(out.FormattedBody, "✏️ was: old<br/>1") {
		t.Errorf("unexpected HTML %q", out.FormattedBody)
	}
}

func TestDeletedContent(t *testing.T) {
	text := deletedContent(&event.MessageEventContent{MsgType: event.MsgText, Body: "hi"}, "🗑 deleted in Threads")
	if text.Body != "~~hi~~\n🗑 deleted in Threads" || !strings.Contains(text.FormattedBody, "<del>hi</del>") {
		t.Errorf("unexpected text content %+v", text)
	}
	media := deletedContent(&event.MessageEventContent{MsgType: event.MsgImage, Body: "a.jpg", URL: "mxc://x/y"}, "🗑 deleted in Threads")
	if media.Body != "a.jpg (🗑 deleted in Threads)" || media.URL != "mxc://x/y" {
		t.Errorf("media should keep its file: %+v", media)
	}
}

func TestPrivacySettings(t *testing.T) {
	meta := &metaid.UserLoginMetadata{}
	if !meta.KeepDeletedEnabled() || !meta.ShowEditsEnabled() || meta.Invisible || meta.Paused {
		t.Error("unexpected defaults")
	}
	off := false
	meta.KeepDeleted, meta.Invisible, meta.Paused = &off, true, true
	updated := &metaid.UserLoginMetadata{}
	meta.CopyFrom(updated)
	if meta.KeepDeletedEnabled() || !meta.Invisible || meta.Paused {
		t.Errorf("re-login should keep settings but not pause: %+v", meta)
	}
	if v, ok := boolArg([]string{"OFF"}); !ok || v {
		t.Error("boolArg off")
	}
	if _, ok := boolArg([]string{"maybe"}); ok {
		t.Error("boolArg accepted garbage")
	}
}
