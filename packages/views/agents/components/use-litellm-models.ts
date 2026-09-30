"use client";

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { useCurrentWorkspace } from "@multica/core/paths";
import {
  liteLLMConnectionOptions,
  liteLLMModelsOptions,
} from "@multica/core/litellm/queries";
import type { RuntimeModel } from "@multica/core/types";

export const LITELLM_PROVIDER = "LiteLLM";

// Model aliases published by the workspace's LiteLLM gateway, shaped like
// runtime models so the picker can list them next to the runtime's own. Empty
// when there is no workspace or no working connection, so the picker degrades
// to its runtime-only behaviour.
export function useLiteLLMModels(): RuntimeModel[] {
  const wsId = useCurrentWorkspace()?.id ?? "";
  const connection = useQuery({
    ...liteLLMConnectionOptions(wsId),
    enabled: wsId !== "",
  });
  const connected = connection.data?.connected === true;
  const models = useQuery(liteLLMModelsOptions(wsId, connected));

  return useMemo(
    () =>
      (models.data?.models ?? []).map((m) => ({
        id: m.id,
        label: m.name || m.id,
        provider: LITELLM_PROVIDER,
      })),
    [models.data],
  );
}
