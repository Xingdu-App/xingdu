import { createContext, useContext } from "react";
export const AvatarContext = createContext({
  image: "",
  loading: true,
  busy: false,
  update: async (_image: string | null): Promise<void> => {
    throw new Error("Avatar provider missing");
  },
});
export const useAvatar = () => useContext(AvatarContext);
