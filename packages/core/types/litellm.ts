export interface LiteLLMConnection {
  connected: boolean;
  configured: boolean;
  can_manage: boolean;
  base_url?: string;
  skill_count: number;
  mcp_count: number;
}

export interface ConnectLiteLLMRequest {
  base_url: string;
  api_key: string;
}

export interface LiteLLMSkill {
  id: string;
  name: string;
  version: string;
  description: string;
  import_url: string;
}

export interface ListLiteLLMSkillsResponse {
  skills: LiteLLMSkill[];
}

export interface LiteLLMMCPServer {
  server_name: string;
  alias: string;
  description: string;
  transport: string;
}

export interface ListLiteLLMMCPServersResponse {
  servers: LiteLLMMCPServer[];
}
