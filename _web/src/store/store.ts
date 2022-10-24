import { writable } from "svelte/store";

const ui = {
  "sidebar": "",
};

const head = "";

export const storeData = writable(ui);
export const storeHead = writable(head);

const info = {
  "name": "",
  "version": "",
  "environment": "",
  "buildDate": "",
  "buildCommit": "",
  "startDate": "",
  "serverDate": "",
};

export const storeInfo = writable(info);
