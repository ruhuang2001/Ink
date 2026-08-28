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
      description: "Review device bindings, schedules, and recent print history.",
      navHint: "Devices and jobs",
    },
    prints: {
      label: "Prints",
      title: "Prints",
      description: "Manage pending items, schedules, and print history in one place.",
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
      schedules: {
        morning: {
          title: "Morning digest",
          source: "Morning feed",
        },
        night: {
          title: "Good night reminder",
          source: "Bedtime note",
        },
        weekend: {
          title: "Weekend list",
          source: "Home plan",
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
      sources: {
        worth: {
          name: "Worth reading today",
          type: "RSS",
          note: "Daily article summary",
        },
        weather: {
          name: "Weather reminder",
          type: "Online service",
          note: "Morning weather brief",
        },
        calendar: {
          name: "Family calendar",
          type: "Calendar",
          note: "Recent sync failed. Re-authorize to continue.",
        },
      },
    },
    summary: {
      welcomeLabel: "Shape content and get ready to print",
      boundDevices: "Bound devices",
      enabledSchedules: "Enabled schedules",
      completedToday: "Completed today",
      deviceCount: "{count}",
      scheduleCount: "{count}",
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
      scheduleUpdated: "Updated the scheduled task status.",
      scheduleCreated: "Created a new scheduled task.",
      scheduleDeviceUpdated: "Updated the schedule device.",
      scheduleDeleted: "Scheduled task deleted.",
      sourceConnected: "Plugin connected.",
      sourceDisconnected: "Plugin disconnected.",
      pluginUploaded: "Plugin uploaded and installed.",
      pluginInstalledFromGit: "Pulled the plugin repository and finished installation.",
      pluginDisabled: "Plugin disabled.",
      pluginTestPassed: "Plugin connection test passed.",
      pluginTestFailed: "Plugin connection test failed.",
      pluginConfigSaved: "Plugin configuration saved.",
      pluginConfigEnabled: "Plugin configuration saved and enabled.",
      defaultDeviceUpdated: "Default device updated.",
      themeUpdated: "Theme preference updated.",
      sendConfirmationEnabled: "New content will go straight into the print queue.",
      tutorialTabShown: "The tutorial tab is now visible.",
      tutorialTabHidden: "The tutorial tab is now hidden.",
      loginProtectionEnabled: "The app will require sign-in again after the browser closes.",
      loginProtectionDisabled: "The sign-in state will be kept after the browser closes.",
      aiConfigSaved: "AI service configuration saved.",
      loginSuccess: "Signed in successfully.",
      passwordUpdated: "Password updated. Please sign in again.",
      feedbackSubmitted: "Feedback sent. The author will receive it directly on paper.",
      loggedOut: "Signed out of the current account.",
      accountCreated: "Created the new account.",
    },
    errors: {
      syncPrintStatus: "Unable to sync print status. Please try again later.",
      syncWorkspace: "Unable to sync workspace data. Please try again later.",
      loadAccountData: "Unable to load account data. Please try again later.",
      loadIntegrations:
        "Unable to load plugins, devices, and AI configuration. Please try again later.",
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
      scheduleDeviceRequired: "Choose an available device first.",
      pluginSourceRequired: "Choose a plugin source first.",
      createSchedule: "Failed to create the scheduled task. Please try again later.",
      updateSchedule: "Failed to update the scheduled task. Please try again later.",
      deleteSchedule: "Failed to delete the scheduled task. Please try again later.",
      authRequired: "Your login session has expired. Please sign in again.",
      uploadPlugin: "Failed to upload the plugin. Please try again later.",
      installPluginFromGit: "Failed to install the plugin from Git. Please try again later.",
      disablePlugin: "Failed to disable the plugin. Please try again later.",
      testPlugin: "Failed to test the plugin connection. Please try again later.",
      savePluginConfiguration: "Failed to save the plugin configuration. Please try again later.",
      defaultDeviceOffline: "An unbound device cannot be set as default.",
      saveAIConfig: "Failed to save the AI configuration. Please try again later.",
      loadAccountDataRelogin: "Unable to load account data. Please sign in again.",
      login: "Sign-in failed. Please try again later.",
      changePassword: "Failed to change the password. Please try again later.",
      feedbackLoginRequired: "Sign in before sending feedback.",
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
      scheduleTitle: "New scheduled task",
      scheduleManualSource: "Created manually",
      scheduleTimeFallback: "Every day 19:30",
      deviceName: "Memobird {count}",
      devicePendingNote: "Waiting to bind",
      pluginGitRepo: "Git repository: {url}",
      pluginDefaultNote: "Can be used as a scheduled print content source",
      pluginRuntimeNode: "Node",
      pluginRuntimePython: "Python",
      pluginSourceGit: "Git plugin",
      pluginSourceUpload: "Uploaded plugin",
      sourceConnectedNote: "Connected normally",
      sourceDisconnectedNote: "Not connected to this source yet",
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
    demoBanner: {
      body: "Devices, conversations, and prints are demo content right now. Sign in to continue with real data.",
      action: "Log in",
    },
  },
  login: {
    hero: {
      titleLine1: "Open Ink",
      titleLine2: "Pick up your paper-note ideas",
    },
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
    emptyState: {
      title: "No messages yet",
      withHistory: "Start a new conversation by typing some content.",
      firstConversation: "There is no history yet. Type your first message to begin.",
    },
    selection: {
      select: "Select this message",
      deselect: "Deselect this message",
    },
    actions: {
      deleteConversation: "Delete conversation",
      printSelected: "Print selected messages",
      printConversation: "Print current conversation",
      saveDraft: "Save draft",
      viewPrintQueue: "View print queue",
      regenerate: "Regenerate",
      send: "Send",
    },
  },
  status: {
    syncErrorTitle: "Device sync error",
    boundDevices: "Bound devices",
    autoPrint: "Auto print",
    recentPrints: "Recent prints",
    defaultDevicePrefix: "Default · ",
    emptyDevices: {
      title: "No Memobird devices yet",
      authenticated:
        "After you sign in, the binding guide appears automatically. Enter the device ID there to bind a real device.",
      anonymous:
        "This is a demo workspace while signed out. After you sign in, this area switches to your real device list.",
    },
    emptySchedules: "No auto-print schedules yet",
    actions: {
      addDevice: "Add device",
      setDefault: "Set as default",
      remove: "Remove",
      unbind: "Unbind",
      goToPrints: "Go to prints",
      enableTask: "Enable {title}",
      disableTask: "Disable {title}",
    },
    dialog: {
      title: "Add device",
      description: {
        authenticated:
          "After sign-in, the device will be bound to the current account and can be set as default or removed later.",
        anonymous: "In demo mode, added devices are saved only in local sample data.",
      },
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
      title: "The three most common ways to print in Ink",
      subtitle: "Chat to print, direct print, and scheduled print",
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
      schedule: {
        title: "Scheduled print",
        body: "Send content automatically at a fixed time.",
      },
    },
    actions: {
      goToConversations: "Go to conversations",
      goToPrints: "Go to prints",
    },
    stepsSection: {
      eyebrow: "How it works",
      title: "Three common ways to use Ink",
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
      schedule: {
        title: "Use scheduled printing for recurring content",
        body: "Create an automatic task in the print view so specific content is sent to the device on a fixed schedule, such as morning reminders or recurring notices.",
        note: "Best for recurring content you want to print steadily.",
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
      scheduleUse: {
        question: "What is scheduled printing good for?",
        answer:
          "It works well for recurring content you want to send every day or every week, such as reminders, to-do summaries, or recurring greetings.",
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
        title: "Change password",
        description:
          "Password editing stays in a separate dialog. This page only shows the security summary.",
        action: "Change password",
        securityRule: "Security rule",
        securityRuleValue: "New password must be at least 8 characters",
        result: "After submit",
        resultValue: "You will be sent back to the sign-in page to authenticate again",
      },
      passwordDialog: {
        title: "Change password",
        description:
          "Enter your current password and set a new one. After a successful update, you will be sent back to sign in again.",
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
        description:
          "Create separate accounts for members. Each account loads its own workspace after sign-in.",
        action: "Create account",
        accountType: "Account style",
        accountTypeValue: "Use a short username when possible, for example alice",
        initialRole: "Initial role",
        initialRoleValue: "New accounts are created as members by default",
      },
      createAccountDialog: {
        title: "Create account",
        description:
          "Create a separate sign-in account for a member. It syncs into the current workspace immediately after submission.",
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
      syncErrorTitle: "Account sync issue",
      defaultDevice: "Default device",
      noDefaultDevice: "No device selected",
      tutorialTab: "Tutorial tab",
      tutorialTabShown: 'The "Tutorial" tab is shown in the top and bottom navigation.',
      tutorialTabHidden: 'The "Tutorial" tab is hidden in the top and bottom navigation.',
      toggleAria: {
        enableTutorialTab: "Enable tutorial tab",
        disableTutorialTab: "Disable tutorial tab",
      },
    },
    appearance: {
      title: "Appearance",
      currentTheme: "Current theme: {value}",
      description: 'Choose "System" to follow the device light or dark appearance automatically.',
    },
    language: {
      title: "Language",
      current: "Current language: {value}",
      description: "Changes apply immediately on this page and are saved as your preference.",
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
    plugins: {
      testPassed: "Connection test passed.",
      title: "Plugins",
      installed: "Installed plugins",
      add: "Add plugin",
      empty: "No plugins yet",
      configure: "Configure plugin",
      disabling: "Disabling...",
      disable: "Disable",
      addDialog: {
        title: "Add plugin",
        securityTitle: "Install trusted plugins only",
        securityWarning:
          "Plugins and their dependencies execute on the server with the server process privileges. Install only from trusted sources.",
        permissionsWarning:
          "Requested permission badges are declarations, not grants enforced by a sandbox.",
        zipUpload: "Upload ZIP",
        githubImport: "Import from GitHub",
        chooseZip: "Choose ZIP file",
        uploading: "Uploading...",
        processing: "Processing {name}",
        repoUrl: "Repository URL",
        repoRef: "Branch",
        repoSubdir: "Subdirectory",
        placeholders: {
          repoUrl: "Example: https://github.com/example/ink-plugin.git",
          repoRef: "Default: main",
          repoSubdir: "Example: plugins/acme-source",
        },
        importing: "Importing...",
        submit: "Import plugin",
        errors: {
          repoUrlRequired: "Enter the Git repository URL.",
        },
      },
      configDialog: {
        fallbackTitle: "Plugin configuration",
        installStatus: "Install status",
        fetchPolicy: "Fetch policy",
        fetchEveryMinutes: "Fetch every {minutes} minutes",
        nextFetchAt: "Next fetch {time}",
        noFetchScheduled: "No automatic fetch is scheduled right now",
        enableWorkspaceBinding: "Enable binding for this workspace",
        enableWorkspaceBindingHint:
          "After enabling it, this workspace can test the plugin and create related scheduled tasks.",
        noWorkspaceConfig:
          "This plugin has no workspace-level configuration. You can test or save it directly.",
        checkboxFallback: "Enable this option",
        secretPlaceholder: "Leave blank to keep the current secret",
        testing: "Testing...",
        test: "Test plugin",
        saving: "Saving...",
        save: "Save config",
      },
      anonymousActions: {
        connect: "Connect",
        disconnect: "Disconnect",
      },
      permissions: {
        networkNone: "No network",
        networkAll: "All network",
        networkHosts: "Network: {hosts}",
        unspecifiedHosts: "unspecified hosts",
        cache: "Persistent cache",
        installScripts: "Install scripts",
      },
      sourceTypeGit: "GitHub",
      sourceTypeZip: "ZIP",
    },
  },
  prints: {
    syncErrorTitle: "Device status sync error",
    confirmDeleteSchedule: "Delete this scheduled task?",
    recentPrints: "Recent prints",
    connectedPlugins: "Connected plugins",
    moreSettings: "More settings",
    actions: {
      bindingTutorial: "Binding tutorial",
      newPrint: "New print",
      newSchedule: "New schedule",
      confirmPrint: "Confirm print",
      cancelPrint: "Cancel print",
      preview: "Print preview",
    },
    preview: {
      title: "Print preview",
      hint: "This preview approximates the Memobird's 384px-wide black-and-white image output at 22px text size. The physical print may vary slightly by device.",
    },
    pending: {
      title: "Pending prints",
      emptyTitle: "No pending prints",
      emptyAuthenticated:
        "After you bind a device, generate content in conversations first and come back here to confirm whether it should print.",
      emptyAnonymous:
        "Signed-out mode shows a demo stream. After sign-in, this switches to real print history for each account.",
      targetDevice: "Target device",
      queuedHint:
        "Once sent to Memobird, the job can no longer be canceled or rebound to another device.",
    },
    schedules: {
      title: "Schedules",
      emptyTitle: "No schedules yet",
      deviceLabel: "Send to device",
      enableTask: "Enable {title}",
      disableTask: "Disable {title}",
      sendToDevice: "Send to {device}",
      nextRunAt: "Next run {time}",
      batchSizeHint: "Print {count} items per run, ordered by the earliest fetched content first.",
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
      description: "Create a new print item.",
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
    scheduleDialog: {
      title: "New scheduled task",
      manualSourceFallback: "Created manually",
      description: {
        authenticated: "Choose a connected plugin as the source and configure when it should run.",
        anonymous: "Create an automatic printing plan.",
      },
      fields: {
        title: "Task name",
        plugin: "Source plugin",
        frequency: "Frequency",
        timezone: "Timezone",
        hour: "Hour",
        minute: "Minute",
        weekdays: "Run on",
        batchSize: "Items per print",
        source: "Content source",
        time: "Execution time",
        device: "Target device",
      },
      placeholders: {
        title: "Example: Morning reminder",
        time: "Every day 19:30",
        plugin: "Choose a plugin",
        timezone: "Example: Asia/Shanghai",
        source: "Example: Created manually",
      },
      frequency: {
        daily: "Every day",
        weekly: "Every week",
      },
      emptyPlugins:
        "No plugin is available right now. Finish plugin installation and workspace setup in Settings first.",
      pluginDetails: {
        fetchFrequency: "Fetch frequency",
        fetchEveryMinutes: "Fetch every {minutes} minutes",
        fetchHint:
          "Fetching runs independently on the plugin binding. Scheduled tasks only consume already fetched content.",
        fetchStatus: "Fetch status",
        lastFetchedAt: "Last fetched {time}",
        neverFetched: "No fetch has run yet",
        nextFetchAt: "Next fetch {time}",
        noNextFetch: "No automatic fetch is scheduled right now",
        batchSizeHint:
          "Each schedule tick prints up to this many items from the earliest fetched content that has not yet been delivered by this task.",
      },
      emptyPluginSelection:
        "Choose a connected plugin first, then decide how many fetched items this task should print each time.",
      submit: "Create task",
      errors: {
        titleRequired: "Enter a task name.",
        deviceRequired: "Bind a device before creating a scheduled task.",
        pluginRequired: "Choose an enabled plugin source first.",
        weekdayRequired: "Select at least one weekday for a weekly task.",
        invalidBatchSize: "Batch size must be a positive integer.",
        createFailed: "Failed to create the scheduled task.",
        timeRequired: "Enter the execution time.",
      },
    },
    errors: {
      invalidBatchSize: "Batch size must be a positive integer.",
    },
  },
  time: {
    justNow: "Just now",
    minutesAgo: "{count} min ago",
    todayAt: "Today {time}",
    yesterdayAt: "Yesterday {time}",
  },
  weekdays: {
    short: {
      0: "Sun",
      1: "Mon",
      2: "Tue",
      3: "Wed",
      4: "Thu",
      5: "Fri",
      6: "Sat",
    },
  },
  schedule: {
    everyDay: "Every day {time}",
    everyWeek: "{days} {time}",
    listSeparator: ", ",
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
    pluginInstallation: {
      installing: "Installing",
      ready: "Ready",
      failed: "Error",
      disabled: "Disabled",
    },
    pluginBinding: {
      disabled: "Disabled",
      disconnected: "Disconnected",
    },
    userRole: {
      admin: "Admin",
      member: "Member",
    },
  },
  theme: {
    light: "Light",
    dark: "Dark",
    system: "System",
  },
  errors: {
    api: {
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
      invalid_feedback_input: "Enter feedback content.",
      feedback_recipient_missing: "No admin account is available to receive feedback.",
      feedback_printer_missing: "The admin account does not have a default Memobird yet.",
      plugin_not_found: "The requested plugin could not be found.",
      plugin_secret_missing: "The server plugin encryption key is missing.",
      invalid_plugin_input: "Enter a valid plugin configuration.",
      plugin_git_install_disabled: "Installing plugins from Git is disabled on the server.",
      schedule_not_found: "The requested schedule could not be found.",
    },
  },
} as const;

export default enUS;
