---
title: frwrd documentation
hide:
  - toc
---

<div class="frwrd-hero" markdown>

<span class="frwrd-kicker">Part of wrkflw · Lightweight · Open source · Runs on your machine</span>

# Build your own 24/7 AI chief of staff.

frwrd is the gateway part of wrkflw, the control plane for assistant-driven
work. It turns Claude Code, Codex, or Pi into an always-on personal assistant:
message it from iMessage, Telegram, or Slack, give it recurring jobs, and let
it handle work in the background. With the wrkflw mirror enabled, your
conversations flow into your wrkflw inbox as tasks, side by side with anything
else you run.

<p class="frwrd-actions">
  <a class="md-button md-button--primary" href="getting-started/">Set up your assistant</a>
  <a class="md-button" href="#what-can-it-do">What it can do&nbsp; ↓</a>
</p>

<p class="frwrd-install">curl -fsSL https://raw.githubusercontent.com/utachicodes/frwrd/main/install.sh | sh</p>

<ul class="frwrd-signals">
  <li><strong>One small binary</strong>No new agent runtime</li>
  <li><strong>Always available</strong>Handles messages and scheduled jobs</li>
  <li><strong>You stay in control</strong>frwrd state stays on your machine</li>
</ul>

</div>

<section class="frwrd-demo" markdown>

<div class="frwrd-section-heading" markdown>

<span class="frwrd-section-label">Background work</span>

## Give it a task. Get the answer in chat.

Send a message from your phone. frwrd runs your coding agent on your machine and
sends the result back when the work is done.

</div>

<div class="frwrd-chat" markdown="0">
<div class="frwrd-chat-bar"><span>Telegram</span><span><i></i> frwrd is online</span></div>
<div class="frwrd-chat-body">
<div class="frwrd-chat-message frwrd-chat-message--user"><span>You · 18:12</span><p>Every weekday at 8am, run my morning brief and send me the three things that need my attention.</p></div>
<div class="frwrd-chat-status"><span>frwrd → Codex</span><span>Schedule ready for review</span></div>
<div class="frwrd-chat-message frwrd-chat-message--assistant"><span>frwrd · 18:14</span><p>I saved your morning brief for weekdays at 8am. Approve this exact revision to start the schedule.</p></div>
</div>
<div class="frwrd-chat-footer"><span>Delivered in chat</span><span>Conversation saved</span></div>
</div>

</section>

<section class="frwrd-outcomes" id="what-can-it-do" markdown>

<span class="frwrd-section-label">What it does</span>

## A personal assistant that keeps working when you step away.

<div class="frwrd-use-cases" markdown="0">
  <article>
    <span>01</span>
    <h3>Handle background work</h3>
    <p>Ask it to inspect a repository, research a question, or prepare an update. You do not need to keep a terminal open.</p>
  </article>
  <article>
    <span>02</span>
    <h3>Run your daily routines</h3>
    <p>Schedule a morning brief, weekly review, or any other Markdown job and receive the result automatically in chat.</p>
  </article>
  <article>
    <span>03</span>
    <h3>Remember the context</h3>
    <p>Keep conversation history and assistant context between messages instead of explaining the same work again.</p>
  </article>
  <article>
    <span>04</span>
    <h3>Use your existing tools</h3>
    <p>Keep the MCP servers, skills, and integrations already configured in Claude Code, Codex, or Pi. Chats also preserve your configured permissions.</p>
  </article>
</div>

</section>

<section class="frwrd-model" markdown>

<span class="frwrd-section-label">How it works</span>

## One lightweight bridge. Your agent does the work.

<div class="frwrd-steps" markdown="0">
  <div><span>01</span><strong>Message your assistant</strong><p>Use iMessage, Telegram, or Slack from wherever you are.</p></div>
  <div><span>02</span><strong>frwrd starts the work</strong><p>It restores the conversation and runs your chosen coding agent in the background.</p></div>
  <div><span>03</span><strong>Get the result</strong><p>frwrd saves the response and sends it back to the same chat.</p></div>
</div>

frwrd does not replace your coding agent. It handles chat, history, schedules,
approvals, and delivery. Claude Code, Codex, or Pi keeps control of models,
tools, skills, and authentication. Chats preserve configured agent permissions;
Codex and Claude jobs bypass interactive permissions so scheduled work can
finish without an operator.

[See the full architecture](https://github.com/utachicodes/frwrd/blob/main/ARCHITECTURE.md){ .frwrd-inline-link }

</section>

<section class="frwrd-paths" markdown>

<span class="frwrd-section-label">Get started</span>

## Build your AI chief of staff

<div class="grid cards" markdown>

-   :material-rocket-launch-outline:{ .lg .middle } **Run frwrd for the first time**

    ---

    Install the binary, connect one channel, configure a backend, and validate
    the setup.

    [:octicons-arrow-right-24: Quickstart](getting-started.md)

-   :material-message-processing-outline:{ .lg .middle } **Connect your chat**

    ---

    Set up private [iMessage](channels/imessage.md), [Telegram](telegram.md), or
    [Slack](slack.md) conversations with narrow sender allowlists.

    [:octicons-arrow-right-24: Configure channels](configuration.md#channels)

-   :material-account-cog-outline:{ .lg .middle } **Design your assistant**

    ---

    Shape identity, durable context, reusable skills, jobs, and evaluation
    criteria without duplicating instructions or committing secrets.

    [:octicons-arrow-right-24: Design the repository](designing-an-assistant.md)

-   :material-calendar-clock:{ .lg .middle } **Automate recurring work**

    ---

    Write Markdown runbooks, run them manually, or add cron triggers and send
    stored results to your primary chat.

    [:octicons-arrow-right-24: Jobs and schedules](jobs.md)

-   :material-server-security:{ .lg .middle } **Operate it continuously**

    ---

    Choose permissions, inspect local state, and run frwrd under `launchd` or
    `systemd`.

    [:octicons-arrow-right-24: Operations guide](services.md)

</div>

</section>

<section class="frwrd-doc-map" markdown>

<span class="frwrd-section-label">Documentation</span>

## Find what you need

| If you need to… | Read… |
| --- | --- |
| install and run one working channel | [Quickstart](getting-started.md) |
| design identity, context, skills, and jobs | [Designing an assistant](designing-an-assistant.md) |
| understand every TOML setting | [Configuration](configuration.md) |
| add recurring or manual work | [Jobs and schedules](jobs.md) |
| choose backend permissions safely | [Permissions and security](security.md) |
| keep frwrd online after logout or reboot | [Run as a service](services.md) |
| inspect commands and outputs | [CLI reference](reference/cli.md) |
| understand or extend the code | [Architecture](https://github.com/utachicodes/frwrd/blob/main/ARCHITECTURE.md) and [contributing](contributing.md) |

!!! note "Canonical source"

    These pages are generated directly from the Markdown in the repository's
    `docs/` directory. If the site and source ever disagree, update the
    Markdown source and rebuild the site.

</section>
