import { createSubjects } from "@openauthjs/core";
import { auth } from "sst/auth";
import { object, string } from "valibot";

export const session = auth.sessions<{
  user: {
    userID: string;
  };
}>();

export const subjects = createSubjects({
  user: object({
    userID: string(),
  }),
});
