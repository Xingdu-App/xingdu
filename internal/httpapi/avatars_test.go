package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"xingdu.app/xingdu/internal/storage"
)

func TestAvatarPNGValidation(t *testing.T) {
	encode := func(w, h int) []byte {
		var b bytes.Buffer
		if err := png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, w, h))); err != nil {
			t.Fatal(err)
		}
		return b.Bytes()
	}
	url := func(b []byte) string { return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b) }
	clean := encode(256, 256)
	got, err := avatarPNG(url(append(append([]byte{}, clean...), []byte("private trailing metadata")...)))
	if err != nil || !bytes.Equal(got, clean) {
		t.Fatal("valid image must be reencoded without trailing metadata", err)
	}
	for _, input := range []string{"", "data:image/svg+xml;base64,PHN2Zz4=", "data:image/png;base64,???", url([]byte("invalid")), url(encode(513, 1))} {
		if _, err := avatarPNG(input); err == nil {
			t.Fatal("accepted invalid image")
		}
	}
}

type avatarTestStore struct {
	Store
	image    []byte
	accessed string
}

func (s *avatarTestStore) Session(context.Context, string) (storage.User, error) {
	return storage.User{ID: "authenticated-user"}, nil
}
func (s *avatarTestStore) Avatar(_ context.Context, id string) ([]byte, error) {
	s.accessed = id
	return s.image, nil
}
func (s *avatarTestStore) SaveAvatar(_ context.Context, id string, image []byte) error {
	s.accessed = id
	s.image = image
	return nil
}
func TestAvatarHTTPProtection(t *testing.T) {
	s := &avatarTestStore{}
	h := New(s, Options{PublicOrigin: "http://localhost"})
	token := strings.Repeat("a", 64)
	for _, tc := range []struct {
		method              string
		authenticated, csrf bool
		want                int
	}{
		{"GET", false, false, 401}, {"PUT", true, false, 403}, {"DELETE", true, false, 403}, {"GET", true, false, 200}, {"DELETE", true, true, 204},
	} {
		r := httptest.NewRequest(tc.method, "/api/v1/account/avatar", strings.NewReader(`{}`))
		r.Header.Set("Origin", "http://localhost")
		r.Header.Set("X-Xingdu-Request", "1")
		r.Header.Set("Content-Type", "application/json")
		if tc.authenticated {
			r.AddCookie(&http.Cookie{Name: cookieName, Value: token})
		}
		if tc.csrf {
			r.Header.Set("X-CSRF-Token", csrfToken(token))
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("%s: got %d want %d", tc.method, w.Code, tc.want)
		}
	}
	if s.accessed != "authenticated-user" {
		t.Fatal("avatar must use authenticated account")
	}
}
