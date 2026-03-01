# RealMe - Magical Telegram Userbot Agent

This is a powerful Telegram Userbot written in Go, powered by the **GLM-4.6V-Flash** multimodal AI model. It logs into your Telegram account, monitors a specified group, and automatically replies to messages that mention you or reply to you, using advanced visual and textual understanding.

## Features

-   **Userbot Capability**: Logs in as YOU, not as a bot.
-   **Multimodal AI**: Uses GLM-4.6V-Flash to understand text and images.
-   **Smart Monitoring**: Replies only when you are mentioned (`@username`) or replied to.
-   **Group Specific**: Can be configured to monitor a specific group or all groups.

## Prerequisites

-   Go 1.21 or higher (Project uses `context.WithoutCancel`).
-   Telegram App ID and Hash (Get it from [my.telegram.org](https://my.telegram.org)).
-   Zhipu AI (GLM) API Key.

## Setup

1.  **Clone the repository**.
2.  **Copy `.env.example` to `.env`**:
    ```bash
    cp .env.example .env
    ```
3.  **Fill in the `.env` file**:
    -   `TELEGRAM_APP_ID` and `TELEGRAM_APP_HASH`: From my.telegram.org.
    -   `GLM_API_KEY`: Your Zhipu AI API key.
    -   `TARGET_GROUP_ID`: The ID of the group you want to monitor. Set to `0` to monitor all chats (use with caution!). To get a group ID, you can use other bots like `@username_to_id_bot` or enable generic logging in the code temporarily.

## Usage

Run the bot:

```bash
go run main.go
```

On the first run, it will ask for your phone number and the Telegram login code (and 2FA password if enabled) in the terminal.
It will then save the session locally (currently in-memory/default session handling, effectively requiring login on restart unless session storage is configured).

## How it works

1.  The bot connects to Telegram using `gotd`.
2.  It listens for new messages.
3.  If a message is in the target group AND (mentions you OR replies to you):
    -   It extracts the text.
    -   If there's an image, it downloads and processes it.
4.  It sends the content to GLM-4.6V-Flash.
5.  It replies to the message with the AI's response.

## Disclaimer

Self-bots (Userbots) are allowed by Telegram but use them responsibly. Spamming or automated actions can lead to account limitations.
