# wrkflw-sdk

Typed JavaScript client for the wrkflw control-plane API. For custom
agents, scripts, and integrations that would rather import a function
than shell out to the CLI.

```sh
npm install wrkflw-sdk
```

```js
import { createClient } from "wrkflw-sdk";

const wrkflw = createClient({
  baseUrl: "https://wrkflw",
  token: process.env.WRKFLW_API_TOKEN,
});

const queued = await wrkflw.tasks.list({ status: "queued" });
const task = await wrkflw.tasks.claim(queued.tasks[0].id);
await wrkflw.tasks.output(task.id, {
  body: "Done. See the attached report.",
  idempotencyKey: `output-${task.id}`,
});
```

## Surface

- `auth.status()`
- `lists.list/get/create`
- `tasks.list/get/create/claim/status/entries/comment/output`
- `agents.list/get/create`
- `inbox()`
- `gateway.getConfig/saveConfig/pull/enqueueReply/pollReplies`

All methods return parsed JSON and throw `WrkflwError` (with `status`
and `code`) on failure. The token travels as a Bearer credential,
times out after 30 seconds by default, and is redacted from every
error message. Task creation accepts `listId`, `parentId`, and
`idempotencyKey`, mirroring the CLI.
