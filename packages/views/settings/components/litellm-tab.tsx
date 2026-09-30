"use client";

import { useState, type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Check, Download, Loader2, Network, PlugZap, Trash2 } from "lucide-react";
import { api } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import {
  liteLLMConnectionOptions,
  liteLLMKeys,
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
import { SettingsCard, SettingsSection } from "./settings-layout";

export function LiteLLMTab() {
  const { t } = useT("settings");
  const wsId = useWorkspaceId();
  const queryClient = useQueryClient();
  const connectionQuery = useQuery(liteLLMConnectionOptions(wsId));
  const connection = connectionQuery.data;
  const connected = connection?.connected === true;
  const skillsQuery = useQuery(liteLLMSkillsOptions(wsId, connected));
  const mcpQuery = useQuery(liteLLMMCPServersOptions(wsId, connected));
  const localSkillsQuery = useQuery({ ...skillListOptions(wsId), enabled: connected });
  const localMcpQuery = useQuery({ ...workspaceMcpServersOptions(wsId), enabled: connected });
  const [baseURL, setBaseURL] = useState("");
  const [apiKey, setAPIKey] = useState("");
  const [connecting, setConnecting] = useState(false);
  const [disconnectOpen, setDisconnectOpen] = useState(false);
  const [disconnecting, setDisconnecting] = useState(false);
  const [importingSkill, setImportingSkill] = useState<string | null>(null);
  const [importingMCP, setImportingMCP] = useState<string | null>(null);

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

      <SettingsSection title={t(($) => $.litellm.skills_title)} description={t(($) => $.litellm.skills_description)}>
        <HubCard loading={skillsQuery.isPending} failed={skillsQuery.isError} empty={!skillsQuery.data?.skills.length} loadingLabel={t(($) => $.litellm.hub_loading)} failedLabel={t(($) => $.litellm.hub_failed)} emptyLabel={t(($) => $.litellm.skills_empty)}>
          {skillsQuery.data?.skills.map((skill) => {
            const imported = importedSkills.has(skill.name);
            return <HubRow key={skill.id} title={skill.name} metadata={skill.version} description={skill.description} action={connection.can_manage ? <Button variant="outline" size="sm" disabled={imported || importingSkill === skill.id} onClick={() => importSkill(skill.id, skill.import_url)}>{importingSkill === skill.id ? <Loader2 className="size-4 animate-spin" /> : imported ? <Check className="size-4" /> : <Download className="size-4" />}{imported ? t(($) => $.litellm.imported) : t(($) => $.litellm.import_skill)}</Button> : undefined} />;
          })}
        </HubCard>
      </SettingsSection>

      <SettingsSection title={t(($) => $.litellm.mcp_title)} description={t(($) => $.litellm.mcp_description)}>
        <HubCard loading={mcpQuery.isPending} failed={mcpQuery.isError} empty={!mcpQuery.data?.servers.length} loadingLabel={t(($) => $.litellm.hub_loading)} failedLabel={t(($) => $.litellm.hub_failed)} emptyLabel={t(($) => $.litellm.mcp_empty)}>
          {mcpQuery.data?.servers.map((server) => {
            const name = server.alias || server.server_name;
            const imported = importedMCP.has(server.server_name);
            return <HubRow key={server.server_name} title={name} metadata={server.transport} description={server.description} action={connection.can_manage ? <Button variant="outline" size="sm" disabled={imported || importingMCP === server.server_name} onClick={() => importMCP(server.server_name)}>{importingMCP === server.server_name ? <Loader2 className="size-4 animate-spin" /> : imported ? <Check className="size-4" /> : <Download className="size-4" />}{imported ? t(($) => $.litellm.imported) : t(($) => $.litellm.import_mcp)}</Button> : undefined} />;
          })}
        </HubCard>
      </SettingsSection>

      <AlertDialog open={disconnectOpen} onOpenChange={(open) => !disconnecting && setDisconnectOpen(open)}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>{t(($) => $.litellm.disconnect_title)}</AlertDialogTitle><AlertDialogDescription>{t(($) => $.litellm.disconnect_description)}</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel disabled={disconnecting}>{t(($) => $.litellm.cancel)}</AlertDialogCancel><AlertDialogAction onClick={disconnect} disabled={disconnecting}>{disconnecting && <Loader2 className="size-4 animate-spin" />}{t(($) => $.litellm.disconnect)}</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>
    </div>
  );
}

function HubCard({ loading, failed, empty, loadingLabel, failedLabel, emptyLabel, children }: { loading: boolean; failed: boolean; empty: boolean; loadingLabel: string; failedLabel: string; emptyLabel: string; children: ReactNode }) {
  if (loading || failed || empty) return <Card><CardContent className="py-8 text-center text-caption text-muted-foreground">{loading ? <span className="inline-flex items-center gap-2"><Loader2 className="size-4 animate-spin" />{loadingLabel}</span> : failed ? <span role="alert" className="text-destructive">{failedLabel}</span> : emptyLabel}</CardContent></Card>;
  return <SettingsCard>{children}</SettingsCard>;
}

function HubRow({ title, metadata, description, action }: { title: string; metadata?: string; description?: string; action?: ReactNode }) {
  return <div className="flex items-center justify-between gap-4 px-4 py-4"><div className="min-w-0"><div className="flex flex-wrap items-center gap-2"><p className="text-body font-medium">{title}</p>{metadata && <span className="rounded-md bg-muted px-1.5 py-0.5 text-micro text-muted-foreground">{metadata}</span>}</div>{description && <p className="mt-1 line-clamp-2 text-caption leading-5 text-muted-foreground">{description}</p>}</div>{action && <div className="shrink-0">{action}</div>}</div>;
}
