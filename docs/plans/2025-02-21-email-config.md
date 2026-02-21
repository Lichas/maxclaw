# Email Configuration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Create email integration configuration UI for IMAP/SMTP settings, allowing the agent to read and send emails.

**Architecture:** Frontend provides forms for email account configuration (IMAP for receiving, SMTP for sending). Backend Gateway manages email polling and sending via existing email channel implementation.

**Tech Stack:** React forms, Gateway config API, email validation

---

### Task 1: Email Config Types

```typescript
interface EmailConfig {
  enabled: boolean;
  provider: 'gmail' | 'outlook' | 'qq' | 'custom';
  email: string;
  password: string; // or app-specific password
  imapHost: string;
  imapPort: number;
  smtpHost: string;
  smtpPort: number;
  useTLS: boolean;
  checkInterval: number; // minutes
}
```

---

### Task 2: Email Settings UI

- Provider presets (Gmail, Outlook, QQ) with auto-filled server settings
- Custom provider option for manual configuration
- Connection test button
- Polling interval setting

---

### Task 3: Backend Integration

Gateway already has email channel support. UI just needs to update config and trigger connection test.

---

### Task 4: Testing

Test email sending and receiving for each provider type.
