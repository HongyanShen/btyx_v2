import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { Application } from "./app/App";
import "./styles/global.css";

const root = document.getElementById("root");
if (!root) {
  throw new Error("页面初始化失败");
}

createRoot(root).render(
  <StrictMode>
    <Application />
  </StrictMode>,
);
