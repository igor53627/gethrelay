# Setting Up Telegram Notifications for GitHub Actions

## Overview

The GitHub Actions workflow is configured to send Telegram notifications for CI progress. This requires setting up GitHub Secrets.

## Steps

### 1. Add Secrets to GitHub Repository

1. Go to your repository: https://github.com/igor53627/gethrelay
2. Click on **Settings** (top menu)
3. In the left sidebar, click **Secrets and variables** → **Actions**
4. Click **New repository secret**
5. Add the following secrets:

#### Secret 1: TELEGRAM_BOT_TOKEN
- **Name**: `TELEGRAM_BOT_TOKEN`
- **Value**: `8210691021:AAE0EZr3NU2Y4xszYJIWZgPxhdrruE3HW7g`
- Click **Add secret**

#### Secret 2: TELEGRAM_CHAT_ID
- **Name**: `TELEGRAM_CHAT_ID`
- **Value**: `403147`
- Click **Add secret**

### 2. Verify Secrets

After adding both secrets, you should see them listed in the Secrets page:
- ✅ TELEGRAM_BOT_TOKEN
- ✅ TELEGRAM_CHAT_ID

**Important**: The values are hidden for security. You can update them but not view them again.

### 3. Test the Integration

1. Push a commit or create a pull request
2. The workflow will run automatically
3. You should receive Telegram messages:
   - 🚀 When workflow starts
   - ✅ When workflow succeeds
   - ❌ When workflow fails

## Notification Triggers

Notifications are sent for:
- ✅ **Workflow Start**: When CI begins
- ✅ **Success**: When all tests pass
- ❌ **Failure**: When any test fails

## Message Format

### Start Notification
```
🚀 gethrelay CI Started

Commit: abc123...
Branch: main
Author: username
Workflow: Gethrelay Tests

View: [link to workflow run]
```

### Success Notification
```
✅ gethrelay CI Passed

All tests completed successfully!

Commit: abc123...
Branch: main

View: [link to workflow run]
```

### Failure Notification
```
❌ gethrelay CI Failed

Some tests or builds failed. Please check the logs.

Commit: abc123...
Branch: main

View: [link to workflow run]
```

## Troubleshooting

### No notifications received
1. Verify secrets are set correctly in GitHub
2. Check that the Telegram bot token is valid
3. Ensure the chat ID is correct (the bot must be started first in the chat)
4. Check GitHub Actions logs for errors

### Bot not responding
1. Start a conversation with your bot on Telegram
2. Send `/start` to initialize the bot
3. Make sure the bot token is correct

### Security Note
- ⚠️ **Never commit secrets to the repository**
- ✅ Secrets are encrypted and only accessible to GitHub Actions
- ✅ Secrets are masked in logs automatically

## Disabling Notifications

If you want to disable notifications for a specific workflow run:
- Use `workflow_dispatch` with `notify: false` parameter

Or comment out the notification jobs in `.github/workflows/gethrelay-tests.yml`.

