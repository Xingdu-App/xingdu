import { useEffect, useRef, useState } from "react";
import { useAvatar } from "./avatar-context";
import Avatar from "./Avatar";
import { errorMessage } from "./api";
import { t, useLocale } from "./i18n";

export default function AvatarEditor({ username }: { username: string }) {
  useLocale();
  const avatar = useAvatar();
  const fileInput = useRef<HTMLInputElement>(null);
  const [preview, setPreview] = useState<string | null>(null);
  const [processing, setProcessing] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const generation = useRef(0);
  useEffect(
    () => () => {
      generation.current++;
    },
    [],
  );
  const busy = avatar.busy || processing;
  async function choose(file?: File) {
    if (!file) return;
    const current = ++generation.current;
    setError("");
    setNotice("");
    setPreview(null);
    if (
      !["image/png", "image/jpeg", "image/webp"].includes(file.type) ||
      file.size > 5 * 1024 * 1024
    ) {
      setError("请选择不超过 5 MB 的 PNG、JPG 或 WebP 图片。");
      return;
    }
    setProcessing(true);
    let bitmap: ImageBitmap | undefined;
    try {
      bitmap = await createImageBitmap(file);
      if (bitmap.width > 8192 || bitmap.height > 8192)
        throw new Error(t("图片尺寸过大，请选择较小的图片。"));
      const canvas = document.createElement("canvas");
      canvas.width = 256;
      canvas.height = 256;
      const context = canvas.getContext("2d");
      if (!context) throw new Error(t("无法处理图片，请重试。"));
      const size = Math.min(bitmap.width, bitmap.height);
      context.drawImage(
        bitmap,
        (bitmap.width - size) / 2,
        (bitmap.height - size) / 2,
        size,
        size,
        0,
        0,
        256,
        256,
      );
      if (current === generation.current)
        setPreview(canvas.toDataURL("image/png"));
    } catch {
      if (current === generation.current) setError("无法处理图片，请重试。");
    } finally {
      bitmap?.close();
      if (current === generation.current) setProcessing(false);
    }
  }
  async function save(next: string | null) {
    const current = generation.current;
    setError("");
    setNotice("");
    try {
      await avatar.update(next);
      if (current === generation.current) {
        setPreview(null);
        setNotice(next ? "头像已更新。" : "头像已移除，将显示用户名首字母。");
      }
    } catch (e) {
      if (current === generation.current) setError(errorMessage(e));
    }
  }
  return (
    <div className="avatar-editor">
      <Avatar username={username} image={preview ?? undefined} large />
      <div className="avatar-editor-controls">
        <strong>{t("账户头像")}</strong>
        <p className="form-hint">
          {t("PNG、JPG 或 WebP，最大 5 MB；自动居中裁剪为正方形。")}
        </p>
        <input
          ref={fileInput}
          type="file"
          accept="image/png,image/jpeg,image/webp"
          hidden
          disabled={busy}
          onChange={(e) => {
            void choose(e.target.files?.[0]);
            e.target.value = "";
          }}
        />
        <div className="avatar-actions">
          <button
            type="button"
            className="secondary compact"
            disabled={busy}
            onClick={() => fileInput.current?.click()}
          >
            {t("选择图片")}
          </button>
          {preview && (
            <>
              <button
                type="button"
                className="primary compact"
                disabled={busy}
                onClick={() => void save(preview)}
              >
                {t("保存头像")}
              </button>
              <button
                type="button"
                className="secondary compact"
                disabled={busy}
                onClick={() => setPreview(null)}
              >
                {t("取消")}
              </button>
            </>
          )}
          {!preview && avatar.image && (
            <button
              type="button"
              className="text-danger"
              disabled={busy}
              onClick={() => void save(null)}
            >
              {t("移除头像")}
            </button>
          )}
        </div>
        {busy && (
          <p className="form-hint" role="status">
            {t("正在处理…")}
          </p>
        )}
        {error && (
          <p className="form-error" role="alert">
            {t(error)}
          </p>
        )}
        {notice && (
          <p className="form-hint" role="status">
            {t(notice)}
          </p>
        )}
      </div>
    </div>
  );
}
