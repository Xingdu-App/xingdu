package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image/png"
	"net/http"
	"strings"
	"xingdu.app/xingdu/internal/storage"
)

type AvatarStore interface {
	Avatar(context.Context, string) ([]byte, error)
	SaveAvatar(context.Context, string, []byte) error
}

// Decode bounds before allocating pixels, then re-encode to strip metadata and
// reject SVG/HTML/polyglot payloads. The browser sends a resized PNG preview.
func avatarPNG(value string) ([]byte, error) {
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(value, prefix) || len(value) > 1400000 {
		return nil, errors.New("invalid avatar")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil {
		return nil, err
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 512 || cfg.Height > 512 {
		return nil, errors.New("invalid avatar size")
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err = png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
func (a *api) avatarRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/account/avatar", a.require(func(w http.ResponseWriter, r *http.Request, u storage.User, _ string) {
		data, err := a.store.Avatar(r.Context(), u.ID)
		if errors.Is(err, storage.ErrNotFound) {
			reply(w, 200, map[string]any{"data": map[string]string{"image": ""}})
			return
		}
		if err != nil {
			storeError(w, err)
			return
		}
		reply(w, 200, map[string]any{"data": map[string]string{"image": "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)}})
	}))
	mux.HandleFunc("PUT /api/v1/account/avatar", a.require(func(w http.ResponseWriter, r *http.Request, u storage.User, _ string) {
		if !a.attempts.allowLimit("avatar:"+u.ID, 30) {
			failure(w, 429, "rate_limited", "请稍后重试")
			return
		}
		var in struct {
			Image string `json:"image"`
		}
		if !decodeLimit(w, r, &in, 1400100) {
			return
		}
		data, err := avatarPNG(in.Image)
		if err != nil {
			failure(w, 422, "invalid_avatar", "头像图片无效，请重新选择图片")
			return
		}
		if err = a.store.SaveAvatar(r.Context(), u.ID, data); err != nil {
			storeError(w, err)
			return
		}
		reply(w, 200, map[string]any{"data": map[string]string{"image": "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)}})
	}))
	mux.HandleFunc("DELETE /api/v1/account/avatar", a.require(func(w http.ResponseWriter, r *http.Request, u storage.User, _ string) {
		if err := a.store.SaveAvatar(r.Context(), u.ID, nil); err != nil {
			storeError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
}
