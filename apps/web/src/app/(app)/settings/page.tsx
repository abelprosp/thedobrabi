"use client";

import { useQuery, useMutation } from "@tanstack/react-query";
import { api, normalizeArray } from "@/lib/api";
import { toast } from "sonner";
import { useEffect, useState } from "react";
import { PageHeader, PageSkeleton } from "@/components/ui";
import { ROLE_LABELS, planLabel, roleLabel } from "@/lib/labels";
import { ThemeSegmented } from "@/components/theme-toggle";
import { useDashboardThemePreference, useSystemTheme } from "@/components/theme-provider";
import {
  ArrowRight,
  Building2,
  CheckCircle2,
  Palette,
  PlugZap,
  ShieldCheck,
  UserRound,
  Users,
  Workflow,
} from "lucide-react";

const inputCls = "w-full rounded-lg border border-line bg-surface px-3 py-2 text-sm text-ink outline-none focus:border-accent/50";
const selectCls = "rounded-lg border border-line bg-surface px-2 text-sm text-ink outline-none";

export default function SettingsPage() {
  const { theme, setTheme } = useSystemTheme();
  const dashboardTheme = useDashboardThemePreference();
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
    <div className="mx-auto max-w-6xl space-y-6 pb-8">
      <PageHeader
        title="Definições"
        description="Personalize a sua conta, organize a equipa e mantenha o acesso seguro."
        actions={
          <div className="hidden items-center gap-2 rounded-full border border-line bg-surface px-3 py-1.5 text-[12px] text-mute shadow-sm sm:flex">
            <span className="h-2 w-2 rounded-full bg-emerald-500" />
            Conta ativa
          </div>
        }
      />
      <AccountOverview me={me.data} org={org.data} />
      <div className="overflow-x-auto lg:hidden">
        <SettingsNav mobile />
      </div>
      <div className="grid gap-6 lg:grid-cols-[210px_minmax(0,1fr)] lg:items-start">
        <SettingsNav />
        <div className="min-w-0 space-y-5">
      {me.isLoading && <PageSkeleton cards={2} />}
      <Box id="aparencia" title="Aparência" icon={Palette} description="Defina como a TheDobra e os seus dashboards aparecem para si.">
        <p className="mb-3 text-[13px] text-mute">Menus, navegação e restantes páginas. Não muda o canvas dos dashboards.</p>
        <ThemeSegmented label="Tema da aplicação" value={theme} onChange={setTheme} />
        <div className="my-4 border-t border-line" />
        <p className="mb-3 text-[13px] text-mute">Pré-definição do canvas. Cada dashboard pode guardar o seu próprio tema ao clicar em Guardar.</p>
        <ThemeSegmented label="Tema dos dashboards" value={dashboardTheme.theme} onChange={dashboardTheme.setTheme} />
      </Box>
      <Box id="organizacao" title="Organização" icon={Building2} description="Informação da organização e do plano atual.">
        <Row k="Utilizador" v={me.data?.name} />
        <Row k="E-mail" v={me.data?.email} />
        <Row k="Função" v={roleLabel(me.data?.role)} />
        <Row k="Organização" v={org.data?.name} />
        <Row k="Plano" v={planLabel(org.data?.plan)} />
      </Box>
      <div id="marca"><BrandBox org={org} /></div>

      {membersList.length <= 1 && (
        <div className="rounded-2xl border border-indigo-200 bg-indigo-50/60 p-5 shadow-sm">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <div className="text-sm font-semibold text-indigo-900">Convide a sua equipa</div>
              <p className="text-[13px] text-indigo-700">Adicione um administrador, analista ou visualizador para partilhar insights.</p>
            </div>
            <div className="flex flex-wrap gap-2">
              <input
                className={`flex-1 rounded-xl border border-indigo-200 bg-surface px-3 py-2 text-sm text-ink outline-none focus:border-indigo-400`}
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

      <Box id="membros" title="Membros" icon={Users} description="Convide pessoas e controle o nível de acesso aos dados.">
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
              className="rounded border border-line bg-surface px-2 py-1 text-[12px]"
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
              {m.role === "owner" && <option value="owner" disabled>{ROLE_LABELS.owner}</option>}
              <option value="admin">{ROLE_LABELS.admin}</option>
              <option value="analyst">{ROLE_LABELS.analyst}</option>
              <option value="viewer">{ROLE_LABELS.viewer}</option>
            </select>
          </div>
        ))}
      </Box>

      <Box id="espacos" title="Espaços de trabalho" icon={Workflow} description="Separe equipas, projetos e permissões por espaço.">
        <div className="mb-3 flex flex-col gap-2 sm:flex-row">
          <input
            className={`flex-1 ${inputCls}`}
            placeholder="Nome do espaço"
            value={wsName}
            onChange={(e) => setWsName(e.target.value)}
          />
          <button onClick={() => createWs.mutate()} className="min-h-11 rounded-lg border border-line px-3 py-2 text-sm hover:bg-bg sm:min-h-0">
            Criar
          </button>
        </div>
        {workspacesList.map((w) => (
          <div key={w.id} className="border-t border-line py-2 text-sm">
            {w.name} <span className="text-mute">/{w.slug}</span>
          </div>
        ))}
      </Box>

      <Box id="seguranca" title="Segurança da conta" icon={ShieldCheck} description="Proteja o login e administre os métodos de autenticação.">
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
          <div className="mt-2 flex flex-col gap-2 sm:flex-row">
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

      <Box id="sso" title="SSO · SAML 2.0" icon={ShieldCheck} description="Centralize o acesso da equipa com o provedor de identidade da empresa.">
        <p className="mb-3 text-[12px] text-mute">
          ACS: {publicURL}/api/v1/auth/saml/{slug || "…"}/acs
          <br />
          Metadata: {publicURL}/api/v1/auth/saml/{slug || "…"}/metadata
        </p>
        <input className={`mb-2 ${inputCls}`} value={samlName} onChange={(e) => setSamlName(e.target.value)} />
        <textarea
          className="h-28 w-full rounded-lg border border-line bg-surface p-2 font-mono text-[11px] outline-none focus:border-accent/50"
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
      <Box id="scim" title="SCIM 2.0" icon={Users} description="Automatize o provisionamento de utilizadores.">
        <p className="mb-3 text-[12px] text-mute">Provisioning de utilizadores (Okta, Entra, Google Workspace).</p>
        <button onClick={() => scim.mutate()} className="rounded-xl border border-line px-3 py-2 text-sm hover:bg-bg">
          Gerar token SCIM
        </button>
      </Box>

      <Box id="gateway" title="Gateway on-premise" icon={PlugZap} description="Ligue fontes privadas sem expor a base de dados à Internet.">
        <p className="mb-3 text-[12px] text-mute">
          Instale o agente gateway numa VM local para aceder a bases PostgreSQL/MySQL/SQL Server sem expô-las à Internet.
        </p>
        <div className="mb-3 flex flex-col gap-2 sm:flex-row">
          <input
            className={`flex-1 ${inputCls}`}
            placeholder="Nome do gateway (ex. fabrica-lisboa)"
            value={gwName}
            onChange={(e) => setGwName(e.target.value)}
          />
          <button onClick={() => generateGatewayToken.mutate()} className="min-h-11 rounded-lg bg-accent px-3 py-2 text-sm text-white hover:bg-accent-2 sm:min-h-0">
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
      </div>
    </div>
  );
}

function AccountOverview({ me, org }: { me?: any; org?: any }) {
  const initials = (me?.name || me?.email || "TD")
    .split(/\s+/)
    .map((part: string) => part[0])
    .join("")
    .slice(0, 2)
    .toUpperCase();
  return (
    <div className="overflow-hidden rounded-3xl border border-line bg-surface shadow-[var(--shadow-card)]">
      <div className="h-20 bg-gradient-to-r from-primary/15 via-accent/10 to-transparent" />
      <div className="-mt-7 flex flex-col gap-4 px-5 pb-5 sm:flex-row sm:items-end sm:justify-between sm:px-7">
        <div className="flex min-w-0 items-end gap-3">
          <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl border-4 border-surface bg-primary text-lg font-semibold text-white shadow-md">
            {initials || <UserRound size={22} />}
          </div>
          <div className="min-w-0 pb-0.5">
            <p className="truncate text-lg font-semibold tracking-tight text-ink">{me?.name || "A sua conta"}</p>
            <p className="truncate text-[13px] text-mute">{me?.email || "Carregando dados da conta…"}</p>
          </div>
        </div>
        <div className="flex flex-wrap gap-2 text-[12px]">
          <StatusPill icon={CheckCircle2} label={planLabel(org?.plan)} tone="primary" />
          <StatusPill icon={Building2} label={org?.name || "A sua organização"} />
        </div>
      </div>
    </div>
  );
}

function StatusPill({ icon: Icon, label, tone = "neutral" }: { icon: typeof CheckCircle2; label: string; tone?: "primary" | "neutral" }) {
  return (
    <span className={`inline-flex items-center gap-1.5 rounded-full border px-3 py-1.5 font-medium ${tone === "primary" ? "border-primary/15 bg-primary/8 text-primary" : "border-line bg-bg text-mute"}`}>
      <Icon size={13} />
      {label}
    </span>
  );
}

function SettingsNav({ mobile = false }: { mobile?: boolean }) {
  const links = [
    { href: "#aparencia", label: "Aparência", icon: Palette },
    { href: "#organizacao", label: "Organização", icon: Building2 },
    { href: "#membros", label: "Membros", icon: Users },
    { href: "#seguranca", label: "Segurança", icon: ShieldCheck },
    { href: "#sso", label: "Acesso empresarial", icon: ShieldCheck },
    { href: "#gateway", label: "Integrações", icon: PlugZap },
  ];
  return (
    <nav aria-label="Secções das definições" className={mobile ? "w-max" : "hidden lg:sticky lg:top-5 lg:block"}>
      {!mobile && <p className="px-3 pb-2 text-[11px] font-semibold uppercase tracking-[0.12em] text-mute">Nesta página</p>}
      <div className={mobile ? "flex gap-1 pb-1" : "space-y-1"}>
        {links.map(({ href, label, icon: Icon }) => (
          <a key={href} href={href} className="group flex items-center justify-between rounded-xl border border-line bg-surface px-3 py-2.5 text-[13px] text-mute shadow-sm transition hover:bg-surface-2 hover:text-ink lg:border-transparent lg:bg-transparent lg:shadow-none">
            <span className="flex items-center gap-2.5"><Icon size={15} />{label}</span>
            <ArrowRight size={14} className="opacity-0 transition group-hover:translate-x-0.5 group-hover:opacity-100" />
          </a>
        ))}
      </div>
    </nav>
  );
}

function BrandBox({ org }: { org: { data?: any; refetch: () => void } }) {
  const [name, setName] = useState(org.data?.brand_name || "");
  const [logo, setLogo] = useState(org.data?.brand_logo_url || "");
  const [from, setFrom] = useState(org.data?.brand_from_email || "");
  const [domain, setDomain] = useState(org.data?.custom_domain || "");
  useEffect(() => {
    setName(org.data?.brand_name || "");
    setLogo(org.data?.brand_logo_url || "");
    setFrom(org.data?.brand_from_email || "");
    setDomain(org.data?.custom_domain || "");
  }, [org.data?.brand_name, org.data?.brand_logo_url, org.data?.brand_from_email, org.data?.custom_domain]);
  const can = org.data?.whitelabel === true;
  const save = useMutation({
    mutationFn: () =>
      api("/api/v1/organizations/current", {
        method: "PATCH",
        body: JSON.stringify({ brand_name: name, brand_logo_url: logo, brand_from_email: from, custom_domain: domain }),
      }),
    onSuccess: () => {
      toast.success("Marca actualizada");
      org.refetch();
    },
    onError: (e: Error) => toast.error(e.message),
  });
  return (
    <Box title="Marca (white-label)">
      <p className="mb-3 text-[13px] text-mute">
        {can
          ? "Nome, logótipo, remetente e domínio próprio. O domínio passa a ser usado em embed, partilha e e-mails."
          : "White-label está no plano Completo. Pode pré-visualizar os campos, mas só esse plano guarda a marca."}
      </p>
      <div className="space-y-2">
        <input className={inputCls} value={name} onChange={(e) => setName(e.target.value)} placeholder="Nome da marca" disabled={!can} />
        <input className={inputCls} value={logo} onChange={(e) => setLogo(e.target.value)} placeholder="URL do logótipo" disabled={!can} />
        <input className={inputCls} value={from} onChange={(e) => setFrom(e.target.value)} placeholder="remetente@empresa.com" disabled={!can} />
        <input className={inputCls} value={domain} onChange={(e) => setDomain(e.target.value)} placeholder="bi.empresa.com" disabled={!can} />
        <p className="text-[11px] text-mute">Aponte um CNAME deste domínio para a app TheDobra. Embed e partilha passam a usar https://domínio.</p>
        <button
          type="button"
          disabled={!can || save.isPending}
          onClick={() => save.mutate()}
          className="rounded-lg bg-primary px-3 py-2 text-sm font-medium text-white disabled:opacity-50"
        >
          Guardar marca
        </button>
      </div>
    </Box>
  );
}

function Box({
  id,
  title,
  description,
  icon: Icon,
  children,
}: {
  id?: string;
  title: string;
  description?: string;
  icon?: typeof Palette;
  children: React.ReactNode;
}) {
  return (
    <section id={id} className="scroll-mt-5 space-y-2 rounded-2xl border border-line bg-surface p-5 text-sm shadow-sm sm:p-6">
      <div className="mb-4 flex items-start gap-3">
        {Icon && <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary"><Icon size={17} /></div>}
        <div>
          <h2 className="text-sm font-semibold tracking-tight text-ink">{title}</h2>
          {description && <p className="mt-1 text-[12px] leading-relaxed text-mute">{description}</p>}
        </div>
      </div>
      {children}
    </section>
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
