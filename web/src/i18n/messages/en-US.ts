const enUS = {
  app: {
    name: "Ink",
  },
  navigation: {
    conversations: {
      label: "Conversations",
      title: "Conversations",
      description: "Shape your content like a chat, confirm the final copy, then send it to print.",
      navHint: "Draft content",
    },
    status: {
      label: "Devices",
      title: "Devices",
      description: "Review device bindings and recent print history.",
      navHint: "Devices and jobs",
    },
    prints: {
      label: "Prints",
      title: "Prints",
      description: "Manage pending items and print history in one place.",
      navHint: "Print flow",
    },
    tutorial: {
      label: "Tutorial",
      title: "Tutorial",
      description: "Learn how to bind a Memobird, configure AI, and print your first note.",
      navHint: "Guide",
    },
    settings: {
      label: "Settings",
      title: "Settings",
      description: "Adjust your default device, assistant style, and printing preferences.",
      navHint: "Preferences",
    },
    login: {
      title: "Welcome to Ink",
      description:
        "Sign in to manage devices, continue conversations, and send notes to your Memobird.",
    },
  },
  common: {
    actions: {
      back: "Back",
      cancel: "Cancel",
      closeWindow: "Close dialog",
      delete: "Delete",
      dismissNotice: "Dismiss notification",
      hide: "Hide",
      login: "Log in",
      logout: "Log out",
      new: "New",
      save: "Save",
      show: "Show",
      submit: "Submit",
    },
    labels: {
      notSet: "Not set",
      workspace: "Notes workspace",
    },
  },
  store: {
    seed: {
      initialConversation: {
        user: "Help me turn today's reminder into something gentle and printable on a small note.",
        assistant:
          "Sure. You could write: Do not rush today. Finish the one most important thing first, and later remember to buy yourself a warm drink.",
      },
      newConversation: {
        title: "New conversation",
        preview: "Start shaping new content here",
      },
      conversations: {
        today: {
          title: "Today's tasks",
          preview: "Remember to buy milk and tape before work ends",
        },
        birthday: {
          title: "Birthday wish",
          preview: "Trying to write something a little softer",
          user: "I want a gentle birthday wish for a friend.",
          assistant:
            "Happy birthday. I hope this year brings you days where you feel cared for and treated gently.",
        },
        shopping: {
          title: "Shopping list",
          preview: "Eggs, toast, tomatoes, yogurt",
          draft: "Remember to restock the usual food at home.",
          user: "Help me make a cleaner shopping list.",
          assistant: "Eggs, toast, tomatoes, yogurt. Those four should be enough for now.",
        },
      },
      devices: {
        desk: {
          name: "Desk Memobird",
          note: "Default device",
        },
        bedroom: {
          name: "Bedroom Memobird",
          note: "Bedtime reminders",
        },
      },
      printJobs: {
        pending: {
          title: "Good night note",
          source: "Conversation draft",
          content: "Get some rest. You already did well today.",
        },
        queued: {
          title: "Tomorrow morning brief",
          source: "Morning feed",
          content: "Sunny tomorrow morning. Remember to bring water.",
        },
        completedTodo: {
          title: "Today's tasks",
          source: "Manual print",
          content: "Finish the most important thing first.",
        },
        completedShopping: {
          title: "Shopping list",
          source: "Manual print",
          content: "Eggs, toast, tomatoes, yogurt.",
        },
      },
    },
    summary: {
      welcomeLabel: "Shape content and get ready to print",
      boundDevices: "Bound devices",
      completedToday: "Completed today",
      deviceCount: "{count}",
      printCount: "{count}",
    },
    flash: {
      localeUpdated: "Language preference updated.",
      conversationCreated: "Created a new conversation draft.",
      conversationDeleted: "Conversation deleted.",
      draftSaved: "Draft saved.",
      replyGenerated: "Generated a new reply.",
      replyRegenerated: "Generated another version of the reply.",
      printQueuedDirectly: "Sent the content straight to the print queue.",
      printQueued: "Added to the print queue.",
      printCancelled: "Print cancelled.",
      printDeviceUpdated: "Updated the target print device.",
      deviceBound: "Device bound.",
      deviceAdded: "Added a new device.",
      deviceDeleted: "Device deleted.",
      deviceRemoved: "Device removed.",
      defaultDeviceUpdated: "Default device updated.",
      themeUpdated: "Theme preference updated.",
      sendConfirmationEnabled: "New content will go straight into the print queue.",
      tutorialTabShown: "The tutorial tab is now visible.",
      tutorialTabHidden: "The tutorial tab is now hidden.",
      loginProtectionEnabled: "The app will require sign-in again after the browser closes.",
      loginProtectionDisabled: "The sign-in state will be kept after the browser closes.",
      aiConfigSaved: "AI service configuration saved.",
      memoryConfigSaved: "Memory system configuration saved.",
      loginSuccess: "Signed in successfully.",
      passwordUpdated: "Password updated. Please sign in again.",
      feedbackSubmitted: "Feedback sent. The author will receive it directly on paper.",
      loggedOut: "Signed out of the current account.",
      accountCreated: "Created the new account.",
    },
    errors: {
      syncPrintStatus: "Unable to load print status. Please try again later.",
      feedbackOutcomeStorage:
        "This browser cannot save the feedback verification reminder. Contact the author to verify receipt and explicitly acknowledge; reloading or clearing site data may lose the reminder.",
      printOutcomeStorage:
        "This browser cannot save the print check reminder. Check the device and explicitly acknowledge it; reloading or clearing site data may lose the reminder.",
      localDraftStorage:
        "This browser could not preserve your unsynced draft. Download it before leaving.",
      syncWorkspace: "Unable to save data. Please try again later.",
      loadAccountData: "Unable to load account data. Please try again later.",
      loadIntegrations: "Unable to load devices and AI configuration. Please try again later.",
      promptRequired: "Enter the content you want Ink to shape first.",
      regenerateUnavailable: "There is no content available to regenerate right now.",
      generateReply: "Unable to generate a reply right now. Please try again later.",
      regenerateReply: "Failed to regenerate the reply. Please try again later.",
      defaultDeviceRequired: "Bind a Memobird and set it as the default device first.",
      createPrint: "Failed to create the print job. Please try again later.",
      selectMessagesRequired: "Select at least one message first.",
      conversationEmpty: "There is no printable content in the current conversation yet.",
      submitPrint: "Failed to submit the print job. Please try again later.",
      cancelPrint: "Failed to cancel the print job. Please try again later.",
      updatePrintDevice: "Failed to update the print device. Please try again later.",
      deviceIdRequired: "Enter the Memobird device ID.",
      bindDevice: "Failed to bind the device. Please try again later.",
      deleteDevice: "Failed to delete the device. Please try again later.",
      authRequired: "Your login session has expired. Please sign in again.",
      defaultDeviceOffline: "An unbound device cannot be set as default.",
      saveAIConfig: "Failed to save the AI configuration. Please try again later.",
      saveMemoryConfig: "Failed to save the memory system configuration. Please try again later.",
      loadAccountDataRelogin: "Unable to load account data. Please sign in again.",
      login: "Sign-in failed. Please try again later.",
      changePassword: "Failed to change the password. Please try again later.",
      feedbackLoginRequired: "Sign in before sending feedback.",
      feedbackSubmissionUnknown:
        "The feedback submission result is unknown. The author may already have received the print. Contact the author to verify receipt, then explicitly acknowledge before sending again.",
      submitFeedback: "Failed to send feedback. Please try again later.",
      createAccount: "Failed to create the account. Please try again later.",
    },
    labels: {
      conversationUntitled: "New conversation",
      selectedMessagesTitle: "Selected messages",
      selectedMessagesSource: "Selected conversation messages",
      currentConversationSource: "Current conversation",
      manualPrintTitle: "Manual note",
      manualPrintContent: "A new print item has been created. You can keep editing it later.",
      manualPrintSource: "Manual print",
      deviceName: "Memobird {count}",
      removedDevice: "Removed device",
      devicePendingNote: "Waiting to bind",
      messageAuthorUser: "Me",
      messageAuthorAssistant: "Ink",
    },
  },
  mockInk: {
    error: "Unable to generate a reply right now. Please try again later.",
    summary: "Start with the one most important thing, then pause and leave yourself some room.",
    reply:
      "Absolutely. You could print it like this: {prompt}{summary} Leave one blank line so it fits the paper note better.",
  },
  shell: {
    routeLoadError: "This page could not load. Check your connection and retry.",
    feedbackOutcome: {
      checked: "I verified with the author",
      confirm:
        "Have you verified receipt with the author and understood that sending again may create a duplicate print?",
    },
    printOutcome: {
      checked: "I checked the device",
      confirm:
        "Have you checked the device output and understood that submitting again may print a duplicate?",
    },
    draftRecovery: {
      damaged:
        "Some draft backups cannot be restored automatically and have been kept unchanged. Download the original content or explicitly discard it; valid drafts can still be restored.",
      damagedItem: "Damaged backup {number}",
      confirmDiscardDamaged:
        "Delete this damaged backup? You can download its original content first.",
      title: "Unsynced account draft found",
      description:
        "This draft is shown only after signing in to its account. Restore replaces your current conversations and preferences, then saves against the latest server revision. You can download it first.",
      restore: "Restore draft",
      download: "Download unsynced draft",
      discard: "Discard backup",
      confirmRestore: "Replace your current conversations and preferences with this backup?",
      confirmDiscard: "Discard this unsynced backup?",
    },
    syncErrors: {
      title: "Some data could not be synchronized",
      workspace: "Workspace: {message}",
      printers: "Devices and printing: {message}",
      ai: "AI service: {message}",
      retry: "Retry",
      reload: "Reload workspace",
      download: "Download local draft JSON",
      confirmReload:
        "The workspace changed elsewhere. Reloading will discard unsaved changes. Reload now?",
    },
    postLoginTutorial: {
      title: "Bind a device before you start",
      description:
        "The binding guide now lives here. Finish these three steps first so content from conversations can go straight to your default device.",
      stepLabel: "Step {index}",
      actions: {
        later: "Maybe later",
        viewTutorial: "View tutorial",
      },
      steps: {
        powerOn: {
          title: "Double-press the power button to print the status slip",
          detail:
            "Only use the Device ID line. Do not copy the WiFi Name or MAC Address as the device ID.",
        },
        bind: {
          title: "Go back to Ink and paste the full Device ID into Add Device",
          detail:
            "You can name the device however you like, but the Device ID is what actually makes binding succeed.",
        },
        default: {
          title: "Set it as default after binding, then run a test print",
          detail:
            "After that, new content from the conversation page can go straight into the print queue with the current default settings.",
        },
      },
    },
  },
  login: {
    form: {
      title: "Sign in",
      accountLabel: "Account",
      accountPlaceholder: "admin",
      passwordLabel: "Password",
      passwordPlaceholder: "Enter your password",
      loggingIn: "Signing in...",
    },
    notice: {
      passwordUpdated: "Password updated. Sign in again with the new password.",
    },
    errors: {
      missingCredentials: "Enter both your account and password.",
    },
  },
  feedback: {
    card: {
      title: "Problems / Suggestions / Rants",
      description:
        "Feedback here notifies the author directly. Use it for issues, ideas, or complaints.",
      action: "Send feedback",
    },
    dialog: {
      title: "Feedback",
      description:
        "Use this form for issues, suggestions, or complaints. The author will be notified.",
      contentLabel: "Feedback",
      placeholder: "Feedback (features / suggestions / complaints)",
      submit: "Submit feedback",
      submitting: "Sending...",
    },
    errors: {
      required: "Enter your feedback first.",
    },
  },
  conversations: {
    recent: "Recent conversations",
    currentConversation: "Current conversation",
    defaultDevice: "Default device: {value}",
    confirmDelete: 'Delete "{title}"?',
    generating: "Drafting a new reply...",
    draftPlaceholder: "Send a message...",
    selectedCount: "{count} selected",
    sending: "Generating...",
    shortcut: "Enter for a new line · Ctrl / ⌘ + Enter to send",
    messageRegion: "Conversation messages",
    save: {
      saving: "Saving…",
      saved: "Saved",
      failed: "Save failed · Retry",
    },
    emptyState: {
      title: "No messages yet",
      withHistory: "Start a new conversation by typing some content.",
      firstConversation: "There is no history yet. Type your first message to begin.",
    },
    selection: {
      message: "Select message {index} for printing",
      select: "Select this message",
      deselect: "Deselect this message",
    },
    actions: {
      deleteConversation: "Delete conversation",
      selectForPrint: "Select to print",
      finishSelection: "Done",
      more: "More conversation actions",
      jumpToLatest: "Jump to latest",
      printSelected: "Print selected messages",
      printConversation: "Print current conversation",
      saveDraft: "Save draft",
      viewPrintQueue: "View print queue",
      regenerate: "Regenerate",
      send: "Send",
    },
  },
  status: {
    boundDevices: "Bound devices",
    recentPrints: "Recent prints",
    defaultDevicePrefix: "Default · ",
    emptyDevices: {
      title: "No Memobird devices yet",
      authenticated:
        "After you sign in, the binding guide appears automatically. Enter the device ID there to bind a real device.",
      anonymous:
        "This is a demo workspace while signed out. After you sign in, this area switches to your real device list.",
    },
    actions: {
      addDevice: "Add device",
      setDefault: "Set as default",
      remove: "Remove",
      unbind: "Unbind",
    },
    dialog: {
      title: "Add device",
      fields: {
        name: "Device name",
        note: "Device note",
        identifier: "Memobird device ID",
        setDefault: "Set as default device",
      },
      placeholders: {
        name: "Example: Living room Memobird",
        note: "Example: Printer by the window",
        identifier: "Example: xxxxxx",
      },
      errors: {
        nameRequired: "Enter a device name.",
        identifierRequired: "Enter the Memobird device ID.",
        createFailed: "Failed to bind the device.",
      },
    },
  },
  tutorial: {
    hero: {
      eyebrow: "Tutorial",
      title: "The two most common ways to print in Ink",
      subtitle: "Chat to print and direct print",
    },
    features: {
      chat: {
        title: "Conversation view",
        body: "Refine content as you chat, then send it straight to print.",
      },
      print: {
        title: "Print view",
        body: "Create a note manually when you already know what to print.",
      },
    },
    actions: {
      goToConversations: "Go to conversations",
      goToPrints: "Go to prints",
    },
    stepsSection: {
      eyebrow: "How it works",
      title: "Two common ways to use Ink",
    },
    steps: {
      chat: {
        title: "Print after refining content through chat",
        body: "Type whatever comes to mind and Ink turns it into a shorter note that fits printing better. You can keep editing in the conversation until it feels right, then print it directly.",
        note: "Best for reminders, notes, encouragement, and short lists.",
      },
      print: {
        title: "Create and print directly from the print view",
        body: "If you already know the content, go straight to the print view and create a note manually without starting in chat first.",
        note: "Best for a quick fixed sentence or a temporary notice.",
      },
    },
    mobile: {
      eyebrow: "Use it like a mobile app",
      title: "Add Ink to the iPhone home screen",
      body: 'Open Ink in Safari, tap the share button, then choose "Add to Home Screen". After that you can launch Ink like a normal app, with top and bottom navigation aligned to the mobile safe area.',
    },
    faqSection: {
      title: "FAQ",
      missingQuestionTitle: "Still have a question?",
      missingQuestionBody:
        "You can send your question, suggestion, or complaint directly to the author.",
    },
    faqs: {
      chatOrPrint: {
        question: "Should I start from the conversation view or the print view?",
        answer:
          "If you are still shaping the content and want Ink to polish it, start in conversations. If the content is already fixed and you only want to print it immediately, go straight to prints.",
      },
      contentTooLong: {
        question: "What if the conversation content is too long to print well?",
        answer:
          "Keep asking follow-up questions and Ink can compress it into shorter, more printable content before you send it to print.",
      },
      notPrinted: {
        question: "Why did the note not print immediately?",
        answer:
          "Check the current status in the print view first. Scheduled items run at their configured time, while manual prints usually start processing quickly.",
      },
    },
    start: {
      title: "Get started",
    },
  },
  settings: {
    navigation: {
      preferences: "Preferences",
      devices: "Devices",
      guide: "Guide",
    },
    account: {
      title: "Account",
      currentAccount: "Current account",
      signedOut: "Not signed in",
      loginProtection: "Sign-in protection",
      loginProtectionEnabled: "You need to sign in again after the browser closes",
      loginProtectionDisabled: "Stay signed in after the browser closes",
      toggleAria: {
        enableLoginProtection: "Enable sign-in protection",
        disableLoginProtection: "Disable sign-in protection",
      },
      passwordCard: {
        title: "Password",
        action: "Change password",
      },
      passwordDialog: {
        title: "Change password",
        currentPassword: "Current password",
        newPassword: "New password",
        confirmPassword: "Confirm new password",
        submitting: "Submitting...",
        submit: "Update password",
        errors: {
          currentPasswordRequired: "Enter your current password.",
          passwordTooShort: "The new password must be at least 8 characters.",
          passwordMismatch: "The new passwords do not match.",
        },
      },
      createAccountCard: {
        title: "Create account",
        action: "Create account",
      },
      createAccountDialog: {
        title: "Create account",
        account: "Account",
        displayName: "Display name",
        initialPassword: "Initial password",
        placeholders: {
          account: "Example: alice",
          displayName: "Example: Alice",
        },
        submitting: "Creating...",
        submit: "Create account",
        errors: {
          accountRequired: "Enter the new account name.",
          passwordTooShort: "The new account password must be at least 8 characters.",
        },
      },
    },
    printing: {
      title: "Printing",
      defaultDevice: "Default device",
      noDefaultDevice: "No device selected",
      tutorialTab: "Tutorial tab",
      toggleAria: {
        enableTutorialTab: "Enable tutorial tab",
        disableTutorialTab: "Disable tutorial tab",
      },
    },
    appearance: {
      title: "Appearance",
    },
    language: {
      title: "Language",
      options: {
        system: "System",
        zhCN: "简体中文",
        enUS: "English",
      },
    },
    ai: {
      title: "AI service",
      loading: "Loading the current AI configuration…",
      configured: "Configured",
      notConfigured: "Not configured",
      edit: "Edit AI config",
      provider: "Provider",
      model: "Model",
      dialog: {
        title: "AI config",
        providerName: "Provider name",
        apiUrl: "API URL",
        defaultModel: "Default model",
        apiKey: "API key",
        placeholders: {
          providerName: "Example: OpenAI Compatible",
          apiUrl: "Example: https://api.openai.com/v1",
          defaultModel: "Example: gpt-4.1-mini",
        },
        apiKeyPlaceholderConfigured: "Leave blank to keep the current server key",
        apiKeyPlaceholderEmpty: "Enter a new server key",
        saving: "Saving...",
        submit: "Save AI config",
        errors: {
          baseUrlRequired: "Enter a compatible API URL.",
          modelRequired: "Enter a default model name.",
          apiKeyRequired: "Enter an API key first.",
        },
      },
    },
    memory: {
      title: "Memory System",
      description: "Optional conversation memory feature, requires Zep service deployment",
      statusAvailable: "Available",
      statusUnavailable: "Service unavailable",
      statusDisabled: "Disabled",
      configure: "Configure memory",
      zepUrl: "Zep service URL",
      apiKey: "API Key",
      notConfigured: "Not configured",
      keyConfigured: "Configured",
      keyNotConfigured: "Not configured (optional)",
      unavailableHint:
        "Memory system is enabled but Zep service is unreachable. Please check the service URL and network.",
      dialog: {
        title: "Memory system config",
        enableMemory: "Enable memory system",
        enableMemoryDescription:
          "When enabled, AI will remember conversations and user preferences",
        zepBaseUrl: "Zep service URL",
        zepApiKey: "Zep API Key (optional)",
        placeholders: {
          zepBaseUrl: "Example: http://zep:8000",
        },
        zepBaseUrlHint:
          "For self-hosting, use internal address; for Zep Cloud, use official API URL",
        zepApiKeyHint: "Leave blank for self-hosting, required for Zep Cloud",
        apiKeyPlaceholderConfigured: "Leave blank to keep current key",
        apiKeyPlaceholderEmpty: "Enter Zep API Key",
        saving: "Saving...",
        submit: "Save config",
        errors: {
          zepBaseUrlRequired: "Zep service URL is required when memory system is enabled.",
        },
      },
    },
  },
  prints: {
    loading: "Loading…",
    historyEmpty: "No print history yet",
    loadMore: "Load more",
    recentPrints: "Recent prints",
    moreSettings: "More settings",
    actions: {
      bindingTutorial: "Binding tutorial",
      newPrint: "New print",
      confirmPrint: "Confirm print",
      cancelPrint: "Cancel print",
      preview: "Print preview",
    },
    preview: {
      error: "Unable to load print content. Please try again.",
      title: "Print preview",
      hint: "This preview approximates the Memobird's 384px-wide black-and-white image output at 22px text size. The physical print may vary slightly by device.",
      canvas: "Paper preview. Scroll to see all content.",
      imageAlt: "Print preview: {title}",
    },
    pending: {
      title: "Pending prints",
      awaitingConfirmation: "Needs confirmation",
      queued: "Queued",
      emptyTitle: "No pending prints",
      emptyAuthenticated:
        "After you bind a device, generate content in conversations first and come back here to confirm whether it should print.",
      emptyAnonymous:
        "Signed-out mode shows a demo stream. After sign-in, this switches to real print history for each account.",
      targetDevice: "Target device",
      queuedHint:
        "Once sent to Memobird, the job can no longer be canceled or rebound to another device.",
    },
    defaultSettings: {
      title: "Default print settings",
      defaultDevice: "Default device",
      notSet: "Not set",
      adjust: "Adjust",
      hint: "If you have not bound a Memobird yet, open the tutorial to get the device ID, then come back to the device page to finish binding.",
    },
    printDialog: {
      title: "New print",
      submit: "Create print",
      fields: {
        title: "Print title",
        content: "Print content",
      },
      placeholders: {
        title: "Example: Good night note",
        content: "Enter the content to print",
      },
      errors: {
        titleRequired: "Enter a print title.",
        contentRequired: "Enter the content to print.",
        createFailed: "Failed to create the print job.",
      },
    },
  },
  time: {
    justNow: "Just now",
    minutesAgo: "{count} min ago",
    todayAt: "Today {time}",
    yesterdayAt: "Yesterday {time}",
  },
  statuses: {
    device: {
      connected: "Connected",
      pending: "Pending",
      offline: "Offline",
    },
    print: {
      pending: "Pending",
      queued: "Queued",
      completed: "Completed",
      cancelled: "Cancelled",
      failed: "Failed",
    },
    source: {
      connected: "Connected",
      error: "Error",
      disconnected: "Disconnected",
    },
    userRole: {
      admin: "Admin",
      member: "Member",
    },
  },
  design: {
    label: "Visual theme",
    paper: "Original",
    structured: "Structured Liquidity",
  },
  theme: {
    light: "Light",
    dark: "Dark",
    system: "System",
  },
  errors: {
    api: {
      auth_refresh_failed:
        "Session refresh failed. The original request was rejected and did not execute. Try again later or sign in again.",
      network_error: "Network error. Check your connection and try again.",
      request_failed: "Request failed. Please try again later.",
      invalid_session_payload: "The login session is invalid. Please sign in again.",
      invalid_credentials: "The account or password is incorrect.",
      forbidden: "Your account does not have permission for this action.",
      ai_not_configured: "AI is not configured yet.",
      ai_secret_missing: "The server AI encryption key is missing.",
      invalid_ai_config: "Enter a valid AI configuration.",
      invalid_ai_input: "Enter valid conversation content.",
      ai_provider_unavailable: "The AI service is temporarily unavailable.",
      printer_not_configured: "The Memobird service is not configured yet.",
      printer_resource_not_found: "The device or print job could not be found.",
      invalid_printer_input: "Enter valid device or print information.",
      printer_unavailable: "The Memobird service is temporarily unavailable.",
      print_submission_unknown:
        "The print submission result is unknown. Check the device before sending again to avoid duplicate prints. The saved task is available in print history.",
      print_content_too_large:
        "Content exceeds the print limits. Split it before previewing or printing: title 512 bytes, body 64 KiB, 8,000 characters total, and image height 8,192 pixels. Avoid excessive consecutive zero-width characters.",
      workspace_conflict:
        "This workspace was updated elsewhere. Your local draft is retained. Download it or reload before saving again.",
      workspace_revision_required:
        "Download any local draft you want to keep, then reload the workspace before saving.",
      invalid_feedback_input: "Enter feedback content.",
      feedback_recipient_missing: "No admin account is available to receive feedback.",
      feedback_printer_missing: "The admin account does not have a default Memobird yet.",
      unauthorized: "Your session has expired. Please sign in again.",
      rate_limited: "Too many attempts. Please try again later.",
      invalid_request: "The request format is invalid. Check it and try again.",
      request_too_large: "The request is too large. Shorten it and try again.",
      invalid_password: "The password must be at least 8 characters long.",
      invalid_profile: "Enter valid account information.",
      email_taken: "This account already exists.",
      user_disabled: "This account has been disabled.",
      current_password_incorrect: "The current password is incorrect.",
      internal_error: "The service is temporarily unavailable. Please try again later.",
      missing_encryption_key: "The server has not configured an encryption key.",
      service_unavailable: "The service is temporarily unavailable. Please try again later.",
      invalid_memory_config: "Enter a valid memory service URL or API key.",
      memory_secret_unavailable:
        "The server memory encryption key is missing or cannot decrypt the stored API key.",
    },
  },
} as const;

export default enUS;
