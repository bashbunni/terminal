import { secret } from "./secret";
import { domain } from "./dns";
import { database } from "./database";
import { webhook } from "./stripe";
import { bus } from "./bus";
import { email } from "./email";

sst.Linkable.wrap(random.RandomString, (resource) => ({
  properties: {
    value: resource.result,
  },
}));

export const authFingerprintKey = new random.RandomString(
  "AuthFingerprintKey",
  {
    length: 32,
  },
);

const authTable = new sst.aws.Dynamo("AuthTable", {
  fields: {
    pk: "string",
    sk: "string",
  },
  ttl: "expiry",
  primaryIndex: {
    hashKey: "pk",
    rangeKey: "sk",
  },
});

const authFn = new sst.aws.Auth("Auth", {
  authenticator: {
    url: true,
    link: [
      bus,
      secret.StripeSecret,
      database,
      email,
      secret.GithubClientID,
      secret.GithubClientSecret,
      secret.TwitchClientSecret,
      secret.TwitchClientID,
      authTable,
      authFingerprintKey,
    ],
    permissions: [
      {
        actions: ["ses:SendEmail"],
        resources: ["*"],
      },
    ],
    handler: "./packages/functions/src/auth2.handler",
  },
});

export const auth = new sst.cloudflare.Worker("AuthWorker", {
  url: true,
  domain: "auth." + domain,
  handler: "./packages/workers/src/proxy.ts",
  environment: {
    ORIGIN_URL: authFn.url,
    NO_CACHE: "true",
  },
});

const apiFn = new sst.aws.Function("OpenApi", {
  handler: "./packages/functions/src/api/index.handler",
  streaming: !$dev,
  link: [
    bus,
    secret.AirtableSecret,
    secret.StripeSecret,
    secret.ShippoSecret,
    secret.EmailOctopusSecret,
    authFn,
    auth,
    database,
    webhook,
  ],
  url: true,
});

export const api = new sst.cloudflare.Worker("OpenApiWorker", {
  url: true,
  dev: false,
  domain: "openapi." + domain,
  handler: "./packages/workers/src/proxy.ts",
  environment: {
    ORIGIN_URL: apiFn.url,
  },
});

new sst.aws.Cron("InventoryTracker", {
  schedule: "rate(1 day)",
  job: {
    link: [database, secret.StripeSecret],
    handler: "./packages/functions/src/cron/inventory.handler",
  },
});

export const outputs = {
  auth: auth.url,
  openapi: api.url,
};
