// Alert window entry (panel + toast). The window label arrives as `?label=`.
import { mount } from "svelte";
import "../styles/tokens.css";
import Alert from "./Alert.svelte";

const target = document.getElementById("app")!;
target.textContent = "";
mount(Alert, { target });
