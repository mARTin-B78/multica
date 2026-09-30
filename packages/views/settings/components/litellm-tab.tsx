"use client";

import { useState, type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Bot, Check, Cpu, Download, KeyRound, Loader2, Network, PlugZap, Sparkles, Trash2, Upload, Wrench } from "lucide-react";
import { api } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import {
  liteLLMConnectionOptions,
  liteLLMKeys,
  liteLLMModelsOptions,
  liteLLMAgentsOptions,
  liteLLMMCPServersOptions,
  liteLLMSkillsOptions,
} from "@multica/core/litellm";
import {
  skillListOptions,
  workspaceKeys,
  workspaceMcpServersOptions,
} from "@multica/core/workspace";
import { Button } from "@multica/ui/components/ui/button";
import { Card, CardContent } from "@multica/ui/components/ui/card";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@multica/ui/components/ui/tabs";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@multica/ui/components/ui/alert-dialog";
import { useT } from "../../i18n";
import { SettingsCard } from "./settings-layout";

export function LiteLLMTab() {
  const { t } = useT("settings");
  const wsId = useWorkspaceId();
  const queryClient = useQueryClient();
  const connectionQuery = useQuery(liteLLMConnectionOptions(wsId));
  const connection = connectionQuery.data;
  const connected = connection?.connected === true;
  const modelsQuery = useQuery(liteLLMModelsOptions(wsId, connected));
  const agentsQuery = useQuery(liteLLMAgentsOptions(wsId, connected));
  const skillsQuery = useQuery(liteLLMSkillsOptions(wsId, connected));
  const mcpQuery = useQuery(liteLLMMCPServersOptions(wsId, connected));
  const localSkillsQuery = useQuery({ ...skillListOptions(wsId), enabled: connected });
  const localMcpQuery = useQuery({ ...workspaceMcpServersOptions(wsId), enabled: connected });
  const [baseURL, setBaseURL] = useState("");
  const [apiKey, setAPIKey] = useState("");
  const [managementKey, setManagementKey] = useState("");
  const [connecting, setConnecting] = useState(false);
  const [disconnectOpen, setDisconnectOpen] = useState(false);
  const [disconnecting, setDisconnecting] = useState(false);
  const [importingSkill, setImportingSkill] = useState<string | null>(null);
  const [importingMCP, setImportingMCP] = useState<string | null>(null);
  const [savingManagementKey, setSavingManagementKey] = useState(false);
  const [publishingSkill, setPublishingSkill] = useState<string | null>(null);
  const [publishingMCP, setPublishingMCP] = useState<string | null>(null);

  async function connect() {
    if (!baseURL.trim() || !apiKey.trim() || connecting) return;
    setConnecting(true);
    try {
      await api.connectLiteLLM(wsId, { base_url: baseURL.trim(), api_key: apiKey.trim() });
      await queryClient.invalidateQueries({ queryKey: liteLLMKeys.all(wsId) });
      setAPIKey("");
      toast.success(t(($) => $.litellm.toast_connected));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t(($) => $.litellm.toast_connect_failed));
    } finally {
      setConnecting(false);
    }
  }

  async function disconnect() {
    if (disconnecting) return;
    setDisconnecting(true);
    try {
      await api.deleteLiteLLMConnection(wsId);
      await queryClient.invalidateQueries({ queryKey: liteLLMKeys.all(wsId) });
      setDisconnectOpen(false);
      toast.success(t(($) => $.litellm.toast_disconnected));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t(($) => $.litellm.toast_disconnect_failed));
    } finally {
      setDisconnecting(false);
    }
  }

  async function importSkill(id: string, url: string) {
    setImportingSkill(id);
    try {
      await api.importSkill({ url });
      await queryClient.invalidateQueries({ queryKey: workspaceKeys.skills(wsId) });
      toast.success(t(($) => $.litellm.toast_skill_imported));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t(($) => $.litellm.toast_import_failed));
    } finally {
      setImportingSkill(null);
    }
  }

  async function importMCP(serverName: string) {
    setImportingMCP(serverName);
    try {
      await api.importLiteLLMMCPServer(wsId, serverName);
      await queryClient.invalidateQueries({ queryKey: workspaceKeys.mcpServers(wsId) });
      toast.success(t(($) => $.litellm.toast_mcp_imported));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t(($) => $.litellm.toast_import_failed));
    } finally {
      setImportingMCP(null);
    }
  }

  async function saveManagementKey() {
    if (!managementKey.trim() || savingManagementKey) return;
    setSavingManagementKey(true);
    try {
      await api.updateLiteLLMManagementKey(wsId, managementKey.trim());
      setManagementKey("");
      await queryClient.invalidateQueries({ queryKey: liteLLMKeys.connection(wsId) });
      toast.success(t(($) => $.litellm.toast_management_key_saved));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t(($) => $.litellm.toast_management_key_failed));
    } finally {
      setSavingManagementKey(false);
    }
  }

  async function publishSkill(skillId: string) {
    setPublishingSkill(skillId);
    try {
      await api.publishLiteLLMSkill(wsId, skillId);
      await queryClient.invalidateQueries({ queryKey: liteLLMKeys.skills(wsId) });
      toast.success(t(($) => $.litellm.toast_skill_published));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t(($) => $.litellm.toast_publish_failed));
    } finally {
      setPublishingSkill(null);
    }
  }

  async function publishMCP(serverId: string) {
    setPublishingMCP(serverId);
    try {
      await api.publishLiteLLMMCPServer(wsId, serverId);
      await queryClient.invalidateQueries({ queryKey: liteLLMKeys.mcpServers(wsId) });
      toast.success(t(($) => $.litellm.toast_mcp_published));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t(($) => $.litellm.toast_publish_failed));
    } finally {
      setPublishingMCP(null);
    }
  }

  if (connectionQuery.isPending) {
    return <div className="flex items-center gap-2 text-body text-muted-foreground"><Loader2 className="size-4 animate-spin" />{t(($) => $.litellm.loading)}</div>;
  }

  if (connectionQuery.isError) {
    return <p role="alert" className="text-body text-destructive">{t(($) => $.litellm.load_failed)}</p>;
  }

  if (!connected) {
    return (
      <Card>
        <CardContent className="space-y-5">
          <div className="flex items-start gap-3">
            <span className="rounded-lg border bg-muted/40 p-2.5"><PlugZap className="size-5" /></span>
            <div><p className="text-body font-medium">{t(($) => $.litellm.connect_title)}</p><p className="mt-1 text-caption leading-5 text-muted-foreground">{t(($) => $.litellm.connect_description)}</p></div>
          </div>
          {!connection?.configured ? (
            <p className="text-caption text-muted-foreground">{t(($) => $.litellm.not_configured)} <code className="rounded-xs bg-muted px-1 py-0.5 text-micro">MULTICA_LITELLM_SECRET_KEY</code>.</p>
          ) : connection.can_manage ? (
            <>
              <div className="space-y-1.5"><Label htmlFor="litellm-url">{t(($) => $.litellm.url_label)}</Label><Input id="litellm-url" value={baseURL} onChange={(event) => setBaseURL(event.target.value)} placeholder={t(($) => $.litellm.url_placeholder)} disabled={connecting} /></div>
              <div className="space-y-1.5"><Label htmlFor="litellm-key">{t(($) => $.litellm.api_key_label)}</Label><Input id="litellm-key" type="password" value={apiKey} onChange={(event) => setAPIKey(event.target.value)} placeholder={t(($) => $.litellm.api_key_placeholder)} disabled={connecting} /></div>
              <div className="flex justify-end"><Button size="sm" onClick={connect} disabled={connecting || !baseURL.trim() || !apiKey.trim()}>{connecting && <Loader2 className="size-4 animate-spin" />}{connecting ? t(($) => $.litellm.connecting) : t(($) => $.litellm.connect)}</Button></div>
            </>
          ) : <p className="text-caption text-muted-foreground">{t(($) => $.litellm.contact_admin)}</p>}
        </CardContent>
      </Card>
    );
  }

  const importedSkills = new Set((localSkillsQuery.data ?? []).map((skill) => skill.name));
  const importedMCP = new Set((localMcpQuery.data ?? []).map((server) => server.name));

  return (
    <div className="space-y-8">
      <Card className="border-primary/30 bg-primary/[0.025]">
        <CardContent className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex min-w-0 items-center gap-3"><span className="rounded-xl border bg-background p-2.5 text-primary"><Network className="size-5" /></span><div className="min-w-0"><div className="flex items-center gap-2"><p className="text-body font-medium">{t(($) => $.litellm.connected_title)}</p><span className="inline-flex items-center gap-1 rounded-full bg-success/10 px-2 py-0.5 text-micro font-medium text-success"><Check className="size-3" />{t(($) => $.integrations.status_connected)}</span></div><p className="mt-1 truncate text-caption text-muted-foreground">{connection.base_url}</p></div></div>
          {connection.can_manage && <Button variant="outline" size="sm" onClick={() => setDisconnectOpen(true)}><Trash2 className="size-4" />{t(($) => $.litellm.disconnect)}</Button>}
        </CardContent>
      </Card>

      {connection.can_manage && (
        <Card>
          <CardContent className="flex flex-col gap-4 sm:flex-row sm:items-end">
            <div className="flex min-w-0 flex-1 items-start gap-3">
              <span className="rounded-lg border bg-muted/40 p-2"><KeyRound className="size-4" /></span>
              <div className="min-w-0 flex-1 space-y-1.5">
                <Label htmlFor="litellm-management-key">{t(($) => $.litellm.management_key_label)}</Label>
                <Input id="litellm-management-key" type="password" value={managementKey} onChange={(event) => setManagementKey(event.target.value)} placeholder={connection.management_configured ? t(($) => $.litellm.management_key_configured) : t(($) => $.litellm.management_key_placeholder)} disabled={savingManagementKey} />
                <p className="text-caption leading-5 text-muted-foreground">{t(($) => $.litellm.management_key_description)}</p>
              </div>
            </div>
            <Button variant="outline" size="sm" onClick={saveManagementKey} disabled={savingManagementKey || !managementKey.trim()}>{savingManagementKey && <Loader2 className="size-4 animate-spin" />}{connection.management_configured ? t(($) => $.litellm.replace_key) : t(($) => $.litellm.save_key)}</Button>
          </CardContent>
        </Card>
      )}

      <Tabs defaultValue="models" className="gap-5">
        <TabsList className="grid h-auto w-full grid-cols-2 gap-1 p-1 md:grid-cols-4">
          <TabsTrigger value="models" className="min-h-9"><Cpu />{t(($) => $.litellm.models_title)}<HubCount value={connection.model_count} /></TabsTrigger>
          <TabsTrigger value="agents" className="min-h-9"><Bot />{t(($) => $.litellm.agents_title)}<HubCount value={connection.agent_count} /></TabsTrigger>
          <TabsTrigger value="mcp" className="min-h-9"><Wrench />{t(($) => $.litellm.mcp_title)}<HubCount value={connection.mcp_count} /></TabsTrigger>
          <TabsTrigger value="skills" className="min-h-9"><Sparkles />{t(($) => $.litellm.skills_title)}<HubCount value={connection.skill_count} /></TabsTrigger>
        </TabsList>

        <TabsContent value="models" className="space-y-4">
          <HubIntro title={t(($) => $.litellm.models_title)} description={t(($) => $.litellm.models_description)} />
          <HubCard loading={modelsQuery.isPending} failed={modelsQuery.isError} empty={!modelsQuery.data?.models.length} loadingLabel={t(($) => $.litellm.hub_loading)} failedLabel={t(($) => $.litellm.hub_failed)} emptyLabel={t(($) => $.litellm.models_empty)}>
            {modelsQuery.data?.models.map((model) => <HubRow key={model.id} title={model.name} metadata={[model.provider, model.mode].filter(Boolean).join(" · ")} description={model.description} status={t(($) => $.litellm.remote_catalog)} />)}
          </HubCard>
        </TabsContent>

        <TabsContent value="agents" className="space-y-4">
          <HubIntro title={t(($) => $.litellm.agents_title)} description={t(($) => $.litellm.agents_description)} />
          <HubCard loading={agentsQuery.isPending} failed={agentsQuery.isError} empty={!agentsQuery.data?.agents.length} loadingLabel={t(($) => $.litellm.hub_loading)} failedLabel={t(($) => $.litellm.hub_failed)} emptyLabel={t(($) => $.litellm.agents_empty)}>
            {agentsQuery.data?.agents.map((agent) => <HubRow key={agent.id} title={agent.name} metadata={agent.protocol_version ? `A2A ${agent.protocol_version}` : "A2A"} description={agent.description || agent.url} status={t(($) => $.litellm.remote_agent)} />)}
          </HubCard>
        </TabsContent>

        <TabsContent value="mcp" className="space-y-6">
          <HubIntro title={t(($) => $.litellm.mcp_title)} description={t(($) => $.litellm.mcp_description)} />
          <HubGroup title={t(($) => $.litellm.from_litellm)} description={t(($) => $.litellm.from_litellm_mcp)}>
            <HubCard loading={mcpQuery.isPending} failed={mcpQuery.isError} empty={!mcpQuery.data?.servers.length} loadingLabel={t(($) => $.litellm.hub_loading)} failedLabel={t(($) => $.litellm.hub_failed)} emptyLabel={t(($) => $.litellm.mcp_empty)}>
              {mcpQuery.data?.servers.map((server) => {
                const name = server.alias || server.server_name;
                const imported = importedMCP.has(server.server_name);
                return <HubRow key={server.server_id || server.server_name} title={name} metadata={server.transport} description={server.description} action={connection.can_manage ? <Button variant="outline" size="sm" disabled={imported || importingMCP === server.server_name} onClick={() => importMCP(server.server_name)}>{importingMCP === server.server_name ? <Loader2 className="size-4 animate-spin" /> : imported ? <Check className="size-4" /> : <Download className="size-4" />}{imported ? t(($) => $.litellm.imported) : t(($) => $.litellm.import_mcp)}</Button> : undefined} />;
              })}
            </HubCard>
          </HubGroup>
          <HubGroup title={t(($) => $.litellm.from_multica)} description={t(($) => $.litellm.from_multica_mcp)}>
            <HubCard loading={localMcpQuery.isPending} failed={localMcpQuery.isError} empty={!localMcpQuery.data?.length} loadingLabel={t(($) => $.litellm.hub_loading)} failedLabel={t(($) => $.litellm.hub_failed)} emptyLabel={t(($) => $.litellm.local_mcp_empty)}>
              {localMcpQuery.data?.map((server) => <HubRow key={server.id} title={server.name} metadata={server.transport} action={connection.can_manage ? <Button variant="outline" size="sm" disabled={!connection.management_configured || publishingMCP === server.id} onClick={() => publishMCP(server.id)}>{publishingMCP === server.id ? <Loader2 className="size-4 animate-spin" /> : <Upload className="size-4" />}{t(($) => $.litellm.publish)}</Button> : undefined} />)}
            </HubCard>
          </HubGroup>
        </TabsContent>

        <TabsContent value="skills" className="space-y-6">
          <HubIntro title={t(($) => $.litellm.skills_title)} description={t(($) => $.litellm.skills_description)} />
          <HubGroup title={t(($) => $.litellm.from_litellm)} description={t(($) => $.litellm.from_litellm_skills)}>
            <HubCard loading={skillsQuery.isPending} failed={skillsQuery.isError} empty={!skillsQuery.data?.skills.length} loadingLabel={t(($) => $.litellm.hub_loading)} failedLabel={t(($) => $.litellm.hub_failed)} emptyLabel={t(($) => $.litellm.skills_empty)}>
              {skillsQuery.data?.skills.map((skill) => {
                const imported = importedSkills.has(skill.name);
                return <HubRow key={skill.id} title={skill.name} metadata={skill.version} description={skill.description} action={connection.can_manage ? <Button variant="outline" size="sm" disabled={imported || importingSkill === skill.id} onClick={() => importSkill(skill.id, skill.import_url)}>{importingSkill === skill.id ? <Loader2 className="size-4 animate-spin" /> : imported ? <Check className="size-4" /> : <Download className="size-4" />}{imported ? t(($) => $.litellm.imported) : t(($) => $.litellm.import_skill)}</Button> : undefined} />;
              })}
            </HubCard>
          </HubGroup>
          <HubGroup title={t(($) => $.litellm.from_multica)} description={t(($) => $.litellm.from_multica_skills)}>
            <HubCard loading={localSkillsQuery.isPending} failed={localSkillsQuery.isError} empty={!localSkillsQuery.data?.length} loadingLabel={t(($) => $.litellm.hub_loading)} failedLabel={t(($) => $.litellm.hub_failed)} emptyLabel={t(($) => $.litellm.local_skills_empty)}>
              {localSkillsQuery.data?.map((skill) => <HubRow key={skill.id} title={skill.name} description={skill.description} action={connection.can_manage ? <Button variant="outline" size="sm" disabled={!connection.management_configured || publishingSkill === skill.id} onClick={() => publishSkill(skill.id)}>{publishingSkill === skill.id ? <Loader2 className="size-4 animate-spin" /> : <Upload className="size-4" />}{t(($) => $.litellm.publish)}</Button> : undefined} />)}
            </HubCard>
          </HubGroup>
        </TabsContent>
      </Tabs>

      <AlertDialog open={disconnectOpen} onOpenChange={(open) => !disconnecting && setDisconnectOpen(open)}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>{t(($) => $.litellm.disconnect_title)}</AlertDialogTitle><AlertDialogDescription>{t(($) => $.litellm.disconnect_description)}</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel disabled={disconnecting}>{t(($) => $.litellm.cancel)}</AlertDialogCancel><AlertDialogAction onClick={disconnect} disabled={disconnecting}>{disconnecting && <Loader2 className="size-4 animate-spin" />}{t(($) => $.litellm.disconnect)}</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>
    </div>
  );
}

function HubCard({ loading, failed, empty, loadingLabel, failedLabel, emptyLabel, children }: { loading: boolean; failed: boolean; empty: boolean; loadingLabel: string; failedLabel: string; emptyLabel: string; children: ReactNode }) {
  if (loading || failed || empty) return <Card><CardContent className="py-8 text-center text-caption text-muted-foreground">{loading ? <span className="inline-flex items-center gap-2"><Loader2 className="size-4 animate-spin" />{loadingLabel}</span> : failed ? <span role="alert" className="text-destructive">{failedLabel}</span> : emptyLabel}</CardContent></Card>;
  return <SettingsCard>{children}</SettingsCard>;
}

function HubCount({ value }: { value: number }) {
  return <span className="rounded-full bg-background/80 px-1.5 py-0.5 text-micro tabular-nums text-muted-foreground">{value}</span>;
}

function HubIntro({ title, description }: { title: string; description: string }) {
  return <div><h3 className="text-title font-semibold">{title}</h3><p className="mt-1 max-w-3xl text-caption leading-5 text-muted-foreground">{description}</p></div>;
}

function HubGroup({ title, description, children }: { title: string; description: string; children: ReactNode }) {
  return <section className="space-y-3" aria-label={title}><div><h4 className="text-body font-medium">{title}</h4><p className="mt-0.5 text-caption text-muted-foreground">{description}</p></div>{children}</section>;
}

function HubRow({ title, metadata, description, status, action }: { title: string; metadata?: string; description?: string; status?: string; action?: ReactNode }) {
  return <div className="flex flex-col gap-3 px-4 py-4 sm:flex-row sm:items-center sm:justify-between"><div className="min-w-0"><div className="flex flex-wrap items-center gap-2"><p className="break-words text-body font-medium">{title}</p>{metadata && <span className="rounded-md bg-muted px-1.5 py-0.5 text-micro text-muted-foreground">{metadata}</span>}{status && <span className="rounded-full bg-success/10 px-2 py-0.5 text-micro font-medium text-success">{status}</span>}</div>{description && <p className="mt-1 line-clamp-2 break-all text-caption leading-5 text-muted-foreground">{description}</p>}</div>{action && <div className="shrink-0 self-start sm:self-auto">{action}</div>}</div>;
}
