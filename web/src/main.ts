import { createApp } from "vue";

import AppRoot from "@/app/AppRoot.vue";
import i18n from "@/i18n";
import { registerServiceWorker } from "@/pwa";
import router from "@/router";
import { pinia } from "@/stores/pinia";

import "@/styles.css";
import "@/structured.css";
import "@/conversations.css";
import "@/prints.css";
import "@/settings.css";

const app = createApp(AppRoot);

app.use(pinia);
app.use(i18n);
app.use(router);
app.mount("#app");

registerServiceWorker();
