export interface MemorySettings {
  enabled: boolean;
  zepBaseUrl: string;
  keyConfigured: boolean;
  available: boolean;
  updatedAt?: string;
}

export interface UpdateMemorySettingsInput {
  enabled: boolean;
  zepBaseUrl: string;
  zepApiKey?: string;
}
