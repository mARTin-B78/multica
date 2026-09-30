import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";

export const liteLLMKeys = {
  all: (workspaceId: string) => ["litellm", workspaceId] as const,
  connection: (workspaceId: string) => ["litellm", workspaceId, "connection"] as const,
  skills: (workspaceId: string) => ["litellm", workspaceId, "skills"] as const,
  mcpServers: (workspaceId: string) => ["litellm", workspaceId, "mcp-servers"] as const,
};

export function liteLLMConnectionOptions(workspaceId: string) {
  return queryOptions({
    queryKey: liteLLMKeys.connection(workspaceId),
    queryFn: () => api.getLiteLLMConnection(workspaceId),
    enabled: workspaceId !== "",
  });
}

export function liteLLMSkillsOptions(workspaceId: string, enabled = true) {
  return queryOptions({
    queryKey: liteLLMKeys.skills(workspaceId),
    queryFn: () => api.listLiteLLMSkills(workspaceId),
    enabled: enabled && workspaceId !== "",
  });
}

export function liteLLMMCPServersOptions(workspaceId: string, enabled = true) {
  return queryOptions({
    queryKey: liteLLMKeys.mcpServers(workspaceId),
    queryFn: () => api.listLiteLLMMCPServers(workspaceId),
    enabled: enabled && workspaceId !== "",
  });
}
