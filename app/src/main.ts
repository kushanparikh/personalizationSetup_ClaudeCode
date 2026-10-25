// Config window entry.
import { mount } from "svelte";
import "./styles/tokens.css";
import App from "./App.svelte";

const target = document.getElementById("app")!;
target.textContent = "";
const app = mount(App, { target });

export default app;
