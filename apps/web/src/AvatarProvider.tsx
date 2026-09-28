import { useEffect, useRef, useState } from "react";
import type { ReactNode } from "react";
import { getAvatar, removeAvatar, saveAvatar } from "./api";
import { AvatarContext } from "./avatar-context";

export default function AvatarProvider({ children }: { children: ReactNode }) {
  const [image, setImage] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const lifecycle = useRef<AbortController | null>(null);
  const version = useRef(0);
  useEffect(() => {
    const controller = new AbortController();
    lifecycle.current = controller;
    const current = version.current;
    getAvatar(controller.signal)
      .then((data) => {
        if (!controller.signal.aborted && version.current === current)
          setImage(data.image);
      })
      .catch(() => {})
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, []);
  async function update(next: string | null) {
    const controller = lifecycle.current;
    if (!controller || controller.signal.aborted) return;
    ++version.current;
    setBusy(true);
    try {
      if (next === null) {
        await removeAvatar(controller.signal);
        if (!controller.signal.aborted) setImage("");
      } else {
        const result = await saveAvatar(next, controller.signal);
        if (!controller.signal.aborted) setImage(result.image);
      }
    } finally {
      if (!controller.signal.aborted) setBusy(false);
    }
  }
  return (
    <AvatarContext value={{ image, loading, busy, update }}>
      {children}
    </AvatarContext>
  );
}
