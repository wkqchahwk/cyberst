# CyberStrikeAI Robot / Chatbot Guide

This guide explains how to connect CyberStrikeAI to DingTalk, Lark (Feishu), and WeCom so users can interact with the platform from their messaging client instead of the web UI.

## Where to configure it

1. Sign in to the CyberStrikeAI web UI.
2. Open `System Settings`.
3. Go to `Robot Settings`.
4. Enable the platform you want to use and enter its credentials.
5. Save the configuration.
6. Restart CyberStrikeAI so long-lived connections are re-established.

The configuration is stored in the `robots` section of `config.yaml`.

## Supported platforms

| Platform | Integration model |
| --- | --- |
| DingTalk | Stream connection |
| Lark (Feishu) | Stream connection |
| WeCom | HTTP callback plus active send-message API |

## DingTalk setup

Use an enterprise internal app, not a simple group webhook bot.

1. Open the [DingTalk Open Platform](https://open.dingtalk.com).
2. Create or select an enterprise internal application.
3. Copy the `Client ID` and `Client Secret`.
4. Enable the bot capability for the app.
5. Set message reception to `Stream mode`.
6. Grant message-related permissions and publish the app.
7. Paste the credentials into CyberStrikeAI and restart the service.

## Lark setup

1. Open the [Lark Open Platform](https://open.feishu.cn).
2. Create or select an enterprise app.
3. Copy the `App ID` and `App Secret`.
4. Enable the bot capability.
5. Add the `im.message.receive_v1` event subscription.
6. Grant the required message permissions.
7. Publish the app.
8. Paste the credentials into CyberStrikeAI and restart the service.

## WeCom setup

WeCom uses encrypted HTTP callbacks and active message sending.

You need:

- `corp_id`
- `agent_id`
- `token`
- `encoding_aes_key`
- `secret`

Configure the callback URL to point to CyberStrikeAI's WeCom robot endpoint, then add the values above to `config.yaml`.

If message sending fails with an IP allowlist error, add the server's public IP to the app allowlist in the WeCom admin console.

## Common bot commands

Common commands include:

- `help`
- `list`
- `new`
- `current`
- `stop`
- `roles`
- `delete <conversationID>`
- `version`

Any other text is treated as a normal user prompt and sent to the AI workflow.

## Recommended validation flow

1. Finish platform-side setup first.
2. Paste the credentials into CyberStrikeAI.
3. Restart the CyberStrikeAI process.
4. Send a direct message to the bot to confirm it is receiving events.

## Test without the chat client

If the messaging client is unavailable, you can validate the integration by calling the robot test endpoint from an authenticated session and checking the server logs.
