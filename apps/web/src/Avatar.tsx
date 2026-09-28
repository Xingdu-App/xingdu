import { useAvatar } from "./avatar-context";
export default function Avatar({
  username,
  image,
  large = false,
}: {
  username: string;
  image?: string;
  large?: boolean;
}) {
  const current = useAvatar();
  const value = image ?? current.image;
  return (
    <span
      className={`user-avatar${large ? " user-avatar-large" : ""}`}
      aria-hidden="true"
    >
      {value ? (
        <img src={value} alt="" />
      ) : (
        Array.from(username.trim())[0]?.toLocaleUpperCase() || "U"
      )}
    </span>
  );
}
