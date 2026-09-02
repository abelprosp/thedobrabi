"use client";

import { useQuery, useMutation } from "@tanstack/react-query";
import { api, normalizeArray } from "@/lib/api";
import { toast } from "sonner";
import { useState } from "react";
import { PageHeader, PageSkeleton } from "@/components/ui";
import { ROLE_LABELS, planLabel, roleLabel } from "@/lib/labels";

const inputCls = "w-full rounded-lg border border-line bg-white px-3 py-2 text-sm text-ink outline-none focus:border-accent/50";
const selectCls = "rounded-lg border border-line bg-white px-2 text-sm text-ink outline-none";

export default function SettingsPage() {
  const me = useQuery({ queryKey: ["me"], queryFn: () => api<any>("/api/v1/auth/me") });
  const org = useQuery({ queryKey: ["org"], queryFn: () => api<any>("/api/v1/organizations/current") });
  const sso = useQuery({ queryKey: ["sso"], queryFn: () => api<any>("/api/v1/sso/connections") });
  const oauth = useQuery({ queryKey: ["oauth-providers"], queryFn: () => api<any>("/api/v1/auth/oauth/providers") });
  const members = useQuery({ queryKey: ["members"], queryFn: () => api<any>("/api/v1/members") });
  const workspaces = useQuery({ queryKey: ["workspaces"], queryFn: () => api<any>("/api/v1/workspaces") });
  const gateways = useQuery({ queryKey: ["gateway-instances"], queryFn: () => api<any>("/api/v1/gateway/instances") });
  const membersList = normalizeArray(members.data);
  const workspacesList = normalizeArray(workspaces.data);
  const ssoList = normalizeArray(sso.data);
  const gatewaysList = normalizeArray(gateways.data);
  const [meta, setMeta] = useState("");
  const [samlName, setSamlName] = useState("Okta / Entra ID");
  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteRole, setInviteRole] = useState("analyst");
  const [wsName, setWsName] = useState("");
  const [mfaSecret, setMfaSecret] = useState("");
  const [mfaUrl, setMfaUrl] = useState("");
  const [mfaCode, setMfaCode] = useState("");
  const [gwName, setGwName] = useState("");
  const [gwToken, setGwToken] = useState("");

  const saveSaml = useMutation({
    mutationFn: () =>
      api("/api/v1/sso/connections", {
        method: "POST",
        body: JSON.stringify({ kind: "saml", name: samlName, metadata_xml: meta }),
      }),
    onSuccess: () => {
      toast.success("Ligação SAML guardada");
      sso.refetch();
    },
    onError: (e: Error) => toast.error(e.message),
  });
  const scim = useMutation({
    mutationFn: () => api<{ token: string; base_url: string }>("/api/v1/sso/scim-token", { method: "POST" }),
    onSuccess: (d) => toast.success("Token SCIM gerado — copie agora: " + d.token),
    onError: (e: Error) => toast.error(e.message),
  });
  const invite = useMutation({
    mutationFn: () =>
      api<{ invite_url: string }>("/api/v1/members/invite", {
        method: "POST",
        body: JSON.stringify({ email: inviteEmail, role: inviteRole }),
      }),
    onSuccess: (d) => {
      toast.success("Convite enviado");
      navigator.clipboard?.writeText(d.invite_url).catch(() => {});
      setInviteEmail("");
      members.refetch();
    },
    onError: (e: Error) => toast.error(e.message),
  });
  const createWs = useMutation({
    mutationFn: () => api("/api/v1/workspaces", { method: "POST", body: JSON.stringify({ name: wsName }) }),
    onSuccess: () => {
      toast.success("Espaço criado");
      setWsName("");
      workspaces.refetch();
    },
    onError: (e: Error) => toast.error(e.message),
  });
  const generateGatewayToken = useMutation({
    mutationFn: () => api<{ token: string }>("/api/v1/gateway/tokens", { method: "POST", body: JSON.stringify({ name: gwName || "gateway-local" }) }),
    onSuccess: (d) => {
      toast.success("Token de gateway gerado");
      setGwToken(d.token);
      gateways.refetch();
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const slug = org.data?.slug;
  const publicURL = oauth.data?.public_url || "http://localhost:8080";
  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <PageHeader title="Definições" description="Organização, membros, espaços de trabalho e autenticação." />
      {me.isLoading && <PageSkeleton cards={2} />}
      <Box title="Organização">
        <Row k="Utilizador" v={me.data?.name} />
        <Row k="E-mail" v={me.data?.email} />
        <Row k="Função" v={roleLabel(me.data?.role)} />
        <Row k="Organização" v={org.data?.name} />
        <Row k="Plano" v={planLabel(org.data?.plan)} />
      </Box>

      {membersList.length <= 1 && (
        <div className="rounded-2xl border border-indigo-200 bg-indigo-50/60 p-5 shadow-sm">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <div className="text-sm font-semibold text-indigo-900">Convide a sua equipa</div>
              <p className="text-[13px] text-indigo-700">Adicione um administrador, analista ou visualizador para partilhar insights.</p>
            </div>
            <div className="flex flex-wrap gap-2">
              <input
                className={`flex-1 rounded-xl border border-indigo-200 bg-white px-3 py-2 text-sm text-ink outline-none focus:border-indigo-400`}
                placeholder="E-mail do colega"
                value={inviteEmail}
                onChange={(e) => setInviteEmail(e.target.value)}
              />
              <button
                onClick={() => invite.mutate()}
                className="rounded-xl bg-indigo-600 px-4 py-2 text-sm text-white hover:bg-indigo-700"
              >
                Convidar
              </button>
            </div>
          </div>
        </div>
      )}

      <Box title="Membros">
        <div className="mb-3 flex flex-wrap gap-2">
          <input
            className={`flex-1 ${inputCls}`}
            placeholder="E-mail do novo membro"
            aria-label="E-mail do novo membro"
            value={inviteEmail}
            onChange={(e) => setInviteEmail(e.target.value)}
          />
          <select className={selectCls} value={inviteRole} onChange={(e) => setInviteRole(e.target.value)} aria-label="Função do convidado">
            <option value="admin">{ROLE_LABELS.admin}</option>
            <option value="analyst">{ROLE_LABELS.analyst}</option>
            <option value="viewer">{ROLE_LABELS.viewer}</option>
          </select>
          <button onClick={() => invite.mutate()} className="rounded-lg bg-accent px-3 py-2 text-sm text-white hover:bg-accent-2">
            Convidar
          </button>
        </div>
        {membersList.map((m) => (
          <div key={m.id} className="flex items-center justify-between border-t border-line py-2 text-sm">
            <span>
              {m.name} · {m.email}
            </span>
            <select
              className="rounded border border-line bg-white px-2 py-1 text-[12px]"
              value={m.role}
              onChange={async (e) => {
                try {
                  await api(`/api/v1/members/${m.id}`, { method: "PATCH", body: JSON.stringify({ role: e.target.value }) });
                  members.refetch();
                } catch (err: any) {
                  toast.error(err.message);
                }
              }}
            >
              <option value="owner">{ROLE_LABELS.owner}</option>
              <option value="admin">{ROLE_LABELS.admin}</option>
              <option value="analyst">{ROLE_LABELS.analyst}</option>
              <option value="viewer">{ROLE_LABELS.viewer}</option>
            </select>
          </div>
        ))}
      </Box>

      <Box title="Espaços de trabalho">
        <div className="mb-3 flex gap-2">
          <input
            className={`flex-1 ${inputCls}`}
            placeholder="Nome do espaço"
            value={wsName}
            onChange={(e) => setWsName(e.target.value)}
          />
          <button onClick={() => createWs.mutate()} className="rounded-lg border border-line px-3 py-2 text-sm hover:bg-bg">
            Criar
          </button>
        </div>
        {workspacesList.map((w) => (
          <div key={w.id} className="border-t border-line py-2 text-sm">
            {w.name} <span className="text-mute">/{w.slug}</span>
          </div>
        ))}
      </Box>

      <Box title="Autenticação em dois factores">
        <p className="mb-2 text-[12px] text-mute">
          {me.data?.mfa_enabled ? "MFA activo nesta conta." : "Proteja o login com uma app autenticadora (TOTP)."}
        </p>
        {!me.data?.mfa_enabled && (
          <button
            className="rounded-xl border border-line px-3 py-2 text-sm hover:bg-bg"
            onClick={async () => {
              try {
                const d = await api<{ secret: string; otpauth_url: string }>("/api/v1/auth/mfa/enroll", { method: "POST" });
                setMfaSecret(d.secret);
                setMfaUrl(d.otpauth_url);
              } catch (e: any) {
                toast.error(e.message);
              }
            }}
          >
            Gerar segredo
          </button>
        )}
        {mfaSecret && (
          <div className="mt-3 space-y-2">
            <p className="break-all font-mono text-[11px] text-accent">{mfaUrl}</p>
            <p className="text-[12px] text-mute">Segredo: {mfaSecret}</p>
            <input
              className={inputCls}
              placeholder="Código de 6 dígitos"
              value={mfaCode}
              onChange={(e) => setMfaCode(e.target.value)}
            />
            <button
              className="rounded-xl bg-accent px-3 py-2 text-sm text-white hover:bg-accent-2"
              onClick={async () => {
                try {
                  await api("/api/v1/auth/mfa/confirm", { method: "POST", body: JSON.stringify({ code: mfaCode }) });
                  toast.success("MFA activado");
                  me.refetch();
                  setMfaSecret("");
                } catch (e: any) {
                  toast.error(e.message);
                }
              }}
            >
              Confirmar MFA
            </button>
          </div>
        )}
        {me.data?.mfa_enabled && (
          <div className="mt-2 flex gap-2">
            <input
              className={`flex-1 ${inputCls}`}
              placeholder="Código para desactivar"
              value={mfaCode}
              onChange={(e) => setMfaCode(e.target.value)}
            />
            <button
              className="rounded-xl border border-line px-3 py-2 text-sm hover:bg-bg"
              onClick={async () => {
                try {
                  await api("/api/v1/auth/mfa/disable", { method: "POST", body: JSON.stringify({ code: mfaCode }) });
                  toast.success("MFA desactivado");
                  me.refetch();
                } catch (e: any) {
                  toast.error(e.message);
                }
              }}
            >
              Desactivar
            </button>
          </div>
        )}
      </Box>

      <Box title="SSO · SAML 2.0">
        <p className="mb-3 text-[12px] text-mute">
          ACS: {publicURL}/api/v1/auth/saml/{slug || "…"}/acs
          <br />
          Metadata: {publicURL}/api/v1/auth/saml/{slug || "…"}/metadata
        </p>
        <input className={`mb-2 ${inputCls}`} value={samlName} onChange={(e) => setSamlName(e.target.value)} />
        <textarea
          className="h-28 w-full rounded-lg border border-line bg-white p-2 font-mono text-[11px] outline-none focus:border-accent/50"
          placeholder="Cole aqui o metadata XML do IdP"
          value={meta}
          onChange={(e) => setMeta(e.target.value)}
        />
        <button onClick={() => saveSaml.mutate()} className="mt-2 rounded-xl bg-accent px-3 py-2 text-sm text-white hover:bg-accent-2">
          Guardar SAML
        </button>
        <div className="mt-3 space-y-1 text-[12px] text-mute">
          {ssoList.map((c) => (
            <div key={c.id}>
              {c.kind} · {c.name}
            </div>
          ))}
        </div>
      </Box>
      <Box title="SCIM 2.0">
        <p className="mb-3 text-[12px] text-mute">Provisioning de utilizadores (Okta, Entra, Google Workspace).</p>
        <button onClick={() => scim.mutate()} className="rounded-xl border border-line px-3 py-2 text-sm hover:bg-bg">
          Gerar token SCIM
        </button>
      </Box>

      <Box title="Gateway on-premise">
        <p className="mb-3 text-[12px] text-mute">
          Instale o agente gateway numa VM local para aceder a bases PostgreSQL/MySQL/SQL Server sem expô-las à Internet.
        </p>
        <div className="mb-3 flex gap-2">
          <input
            className={`flex-1 ${inputCls}`}
            placeholder="Nome do gateway (ex. fabrica-lisboa)"
            value={gwName}
            onChange={(e) => setGwName(e.target.value)}
          />
          <button onClick={() => generateGatewayToken.mutate()} className="rounded-lg bg-accent px-3 py-2 text-sm text-white hover:bg-accent-2">
            Gerar token
          </button>
        </div>
        {gwToken && (
          <div className="mb-3 rounded-lg bg-indigo-50 p-3 text-[12px] text-indigo-900">
            <span className="font-semibold">Token (copie agora):</span>
            <p className="mt-1 break-all font-mono">{gwToken}</p>
          </div>
        )}
        <div className="space-y-2">
          <div className="text-[12px] text-mute">Instâncias registadas</div>
          {gatewaysList.length === 0 && <p className="text-[12px] text-mute">Ainda sem gateways registados.</p>}
          {gatewaysList.map((g: any) => (
            <div key={g.id} className="flex items-center justify-between border-t border-line py-2 text-sm">
              <span>
                {g.name} · {g.status} · v{g.version || "—"}
              </span>
              <span className="text-[11px] text-mute">{g.last_ping_at ? new Date(g.last_ping_at).toLocaleString("pt-BR") : "nunca"}</span>
            </div>
          ))}
        </div>
      </Box>
    </div>
  );
}

function Box({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="space-y-2 rounded-2xl border border-line bg-surface p-5 text-sm shadow-sm">
      <h2 className="mb-2 text-[13px] text-mute">{title}</h2>
      {children}
    </div>
  );
}
function Row({ k, v }: { k: string; v?: string }) {
  return (
    <div className="flex justify-between border-t border-line py-2 first:border-0">
      <span className="text-mute">{k}</span>
      <span>{v || "—"}</span>
    </div>
  );
}
