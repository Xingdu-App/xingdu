import { renderToString } from "react-dom/server";
import Marketing from "./Marketing";
import type { PublicPage } from "./public-pages";
export { publicPages } from "./public-pages";
export const render = (page: PublicPage) =>
  renderToString(<Marketing page={page} />);

export { blogPosts, blogPath } from "./blog-posts";
