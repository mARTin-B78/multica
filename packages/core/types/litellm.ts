export interface LiteLLMConnection {
  connected: boolean;
  configured: boolean;
  can_manage: boolean;
  management_configured: boolean;
  base_url?: string;
  model_count: number;
  agent_count: number;
  skill_count: number;
  mcp_count: number;
}

export interface LiteLLMModel {
  id: string;
  name: string;
  provider: string;
  mode: string;
  description: string;
}

export interface ListLiteLLMModelsResponse {
  models: LiteLLMModel[];
}

export interface LiteLLMAgent {
  id: string;
  name: string;
  description: string;
  url: string;
  version: string;
  protocol_version: string;
}

export interface ListLiteLLMAgentsResponse {
  agents: LiteLLMAgent[];
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
  server_id: string;
  server_name: string;
  alias: string;
  description: string;
  transport: string;
}

export interface ListLiteLLMMCPServersResponse {
  servers: LiteLLMMCPServer[];
}

export interface LiteLLMPublishedResource {
  id: string;
  name: string;
}
