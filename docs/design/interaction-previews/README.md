# Local Conversation Interaction Verification (2026-10-15)

This verification builds on the existing workspace improvements. No changes were committed or pushed.

- `make check-web` passed: 18 test files and 212 tests; lint, formatting, type checks, build, and performance budgets all passed.
- Guest mode in the browser: desktop screenshots at 1280×633 and mobile screenshots at 390×844 are in this directory.
- Mobile selection mode: at 390px wide, the page has no horizontal overflow; the input area's bottom is around 756px.
- Print selection: selecting the checkbox sets the count to 1 and enables the print button; exiting selection clears it.
- A saved multiline draft shows “Saved”; focusing the input and pressing Ctrl+Enter sends successfully and clears it.
- The computed `white-space` style for message content is `pre-wrap`; the browser reported no script errors.
- `ConversationMessages.spec.ts` covers scroll following, preserving position in reading history, and returning to the latest message.

Physical printers, AI after login, and the real mobile soft keyboard were not tested. Screenshots use local guest-demo data.
