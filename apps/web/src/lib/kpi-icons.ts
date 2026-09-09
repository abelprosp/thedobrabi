import type { LucideIcon } from "lucide-react";
import {
  Activity,
  AlertTriangle,
  Award,
  BadgePercent,
  Banknote,
  BarChart3,
  Bell,
  Bookmark,
  Boxes,
  Building2,
  Calendar,
  CalendarCheck,
  CheckCircle2,
  Clock,
  Cloud,
  Contact,
  CreditCard,
  Database,
  DollarSign,
  Droplet,
  Eye,
  Factory,
  Flag,
  Flame,
  Globe,
  Handshake,
  Heart,
  Home,
  Hourglass,
  Layers,
  Leaf,
  LineChart,
  Lock,
  Mail,
  MapPin,
  Megaphone,
  MessageCircle,
  Package,
  Percent,
  Phone,
  PieChart,
  PiggyBank,
  Receipt,
  Rocket,
  Settings,
  Share2,
  ShieldCheck,
  ShoppingBag,
  ShoppingCart,
  Smile,
  Sparkles,
  Star,
  Store,
  Target,
  ThumbsUp,
  Timer,
  TrendingDown,
  TrendingUp,
  Trophy,
  Truck,
  User,
  UserPlus,
  Users,
  Wallet,
  Warehouse,
  Wrench,
  Zap,
} from "lucide-react";

export type KpiIconItem = {
  name: string;
  label: string;
  group: string;
  Icon: LucideIcon;
  aliases?: string[];
};

export const KPI_ICON_GROUPS = ["Negócio", "Crescimento", "Pessoas", "Tempo", "Lugar", "Comunicação", "Estado"] as const;

export const KPI_ICON_CATALOG: KpiIconItem[] = [
  { name: "dollar-sign", label: "Dinheiro", group: "Negócio", Icon: DollarSign, aliases: ["receita", "real", "usd", "moeda"] },
  { name: "wallet", label: "Carteira", group: "Negócio", Icon: Wallet },
  { name: "credit-card", label: "Cartão", group: "Negócio", Icon: CreditCard },
  { name: "banknote", label: "Notas", group: "Negócio", Icon: Banknote },
  { name: "piggy-bank", label: "Poupança", group: "Negócio", Icon: PiggyBank },
  { name: "receipt", label: "Recibo", group: "Negócio", Icon: Receipt },
  { name: "badge-percent", label: "Percentagem", group: "Negócio", Icon: BadgePercent, aliases: ["desconto", "margem"] },
  { name: "percent", label: "Percentual", group: "Negócio", Icon: Percent },
  { name: "shopping-cart", label: "Carrinho", group: "Negócio", Icon: ShoppingCart, aliases: ["vendas", "pedido"] },
  { name: "shopping-bag", label: "Saco", group: "Negócio", Icon: ShoppingBag },
  { name: "store", label: "Loja", group: "Negócio", Icon: Store },
  { name: "package", label: "Encomenda", group: "Negócio", Icon: Package },
  { name: "boxes", label: "Caixas", group: "Negócio", Icon: Boxes },
  { name: "truck", label: "Entrega", group: "Negócio", Icon: Truck, aliases: ["logistica"] },
  { name: "factory", label: "Fábrica", group: "Negócio", Icon: Factory },
  { name: "trending-up", label: "A subir", group: "Crescimento", Icon: TrendingUp, aliases: ["alta", "crescimento"] },
  { name: "trending-down", label: "A descer", group: "Crescimento", Icon: TrendingDown, aliases: ["queda"] },
  { name: "activity", label: "Actividade", group: "Crescimento", Icon: Activity },
  { name: "line-chart", label: "Linha", group: "Crescimento", Icon: LineChart },
  { name: "bar-chart-3", label: "Barras", group: "Crescimento", Icon: BarChart3 },
  { name: "pie-chart", label: "Pizza", group: "Crescimento", Icon: PieChart },
  { name: "target", label: "Meta", group: "Crescimento", Icon: Target },
  { name: "award", label: "Prémio", group: "Crescimento", Icon: Award },
  { name: "trophy", label: "Troféu", group: "Crescimento", Icon: Trophy },
  { name: "rocket", label: "Foguete", group: "Crescimento", Icon: Rocket },
  { name: "zap", label: "Raio", group: "Crescimento", Icon: Zap },
  { name: "sparkles", label: "Destaque", group: "Crescimento", Icon: Sparkles },
  { name: "users", label: "Equipa", group: "Pessoas", Icon: Users, aliases: ["clientes", "utilizadores"] },
  { name: "user", label: "Pessoa", group: "Pessoas", Icon: User },
  { name: "user-plus", label: "Novo cliente", group: "Pessoas", Icon: UserPlus },
  { name: "contact", label: "Contacto", group: "Pessoas", Icon: Contact },
  { name: "handshake", label: "Acordo", group: "Pessoas", Icon: Handshake },
  { name: "smile", label: "Satisfação", group: "Pessoas", Icon: Smile },
  { name: "heart", label: "Favorito", group: "Pessoas", Icon: Heart },
  { name: "star", label: "Estrela", group: "Pessoas", Icon: Star },
  { name: "thumbs-up", label: "Gosto", group: "Pessoas", Icon: ThumbsUp },
  { name: "calendar", label: "Calendário", group: "Tempo", Icon: Calendar },
  { name: "calendar-check", label: "Data ok", group: "Tempo", Icon: CalendarCheck },
  { name: "clock", label: "Relógio", group: "Tempo", Icon: Clock },
  { name: "timer", label: "Cronómetro", group: "Tempo", Icon: Timer },
  { name: "hourglass", label: "Ampulheta", group: "Tempo", Icon: Hourglass },
  { name: "map-pin", label: "Local", group: "Lugar", Icon: MapPin },
  { name: "globe", label: "Mundo", group: "Lugar", Icon: Globe },
  { name: "building-2", label: "Edifício", group: "Lugar", Icon: Building2, aliases: ["empresa"] },
  { name: "home", label: "Casa", group: "Lugar", Icon: Home },
  { name: "warehouse", label: "Armazém", group: "Lugar", Icon: Warehouse },
  { name: "mail", label: "E-mail", group: "Comunicação", Icon: Mail },
  { name: "message-circle", label: "Mensagem", group: "Comunicação", Icon: MessageCircle },
  { name: "phone", label: "Telefone", group: "Comunicação", Icon: Phone },
  { name: "megaphone", label: "Campanha", group: "Comunicação", Icon: Megaphone },
  { name: "bell", label: "Alerta", group: "Comunicação", Icon: Bell },
  { name: "share-2", label: "Partilhar", group: "Comunicação", Icon: Share2 },
  { name: "circle-check", label: "Concluído", group: "Estado", Icon: CheckCircle2 },
  { name: "triangle-alert", label: "Atenção", group: "Estado", Icon: AlertTriangle },
  { name: "shield-check", label: "Seguro", group: "Estado", Icon: ShieldCheck },
  { name: "lock", label: "Bloqueado", group: "Estado", Icon: Lock },
  { name: "eye", label: "Visível", group: "Estado", Icon: Eye },
  { name: "layers", label: "Camadas", group: "Estado", Icon: Layers },
  { name: "database", label: "Dados", group: "Estado", Icon: Database },
  { name: "cloud", label: "Cloud", group: "Estado", Icon: Cloud },
  { name: "settings", label: "Definições", group: "Estado", Icon: Settings },
  { name: "wrench", label: "Ferramenta", group: "Estado", Icon: Wrench },
  { name: "bookmark", label: "Marcador", group: "Estado", Icon: Bookmark },
  { name: "flag", label: "Bandeira", group: "Estado", Icon: Flag },
  { name: "flame", label: "Fogo", group: "Estado", Icon: Flame },
  { name: "leaf", label: "Folha", group: "Estado", Icon: Leaf },
  { name: "droplet", label: "Gota", group: "Estado", Icon: Droplet },
];

export const KPI_ICON_MAP = new Map(KPI_ICON_CATALOG.map((i) => [i.name, i.Icon]));

export function toKebabIconName(value: string) {
  return value
    .trim()
    .replace(/([a-z0-9])([A-Z])/g, "$1-$2")
    .replace(/([A-Z])([A-Z][a-z])/g, "$1-$2")
    .replace(/_/g, "-")
    .replace(/\s+/g, "-")
    .toLowerCase();
}

export function isKpiIconUrl(value: string) {
  const v = value.replace(/^url:/i, "");
  return /^(https?:\/\/|data:|\/)/i.test(v);
}

export function kpiIconUrl(value: string) {
  return value.replace(/^url:/i, "");
}

export function isLikelyEmoji(value: string) {
  const v = value.trim();
  if (!v || v.length > 8) return false;
  if (/^[a-z0-9-]+$/i.test(v)) return false;
  return true;
}
