#!/usr/bin/env python3
"""Gera CSVs de mock Redorai para cada painel da Loja TheDobra."""

from __future__ import annotations

import csv
import random
from calendar import monthrange
from datetime import date, timedelta
from pathlib import Path

random.seed(42)

MESES = {
    1: "Janeiro",
    2: "Fevereiro",
    3: "Março",
    4: "Abril",
    5: "Maio",
    6: "Junho",
    7: "Julho",
    8: "Agosto",
    9: "Setembro",
    10: "Outubro",
    11: "Novembro",
    12: "Dezembro",
}

EMPRESAS = ["Redorai", "Redorai SP", "Redorai RJ"]
REGIOES = ["Sudeste", "Sul", "Nordeste", "Centro-Oeste", "Norte"]
VENDEDORES = ["Ana Costa", "Bruno Lima", "Carla Souza", "Diego Alves", "Eva Martins", "Fábio Nunes"]
CLIENTES = [
    "Atlas Ltda", "Nimbus SA", "Forge Engenharia", "Helix Saúde", "Orbit Educação",
    "Pulse Varejo", "Lumen Tech", "Verde Agro", "Costa Farma", "Delta Log",
    "Aurora Moda", "Pampa Alimentos", "Serra Café", "Maré Energia", "Céu Soft",
    "Rio Branco", "Casa Norte", "Sul Minas", "Boa Vista", "Terra Nova",
]
PRODUTOS = [
    "Plano Start", "Plano Pro", "Plano Scale", "Add-on API", "Add-on SSO",
    "Consultoria", "Onboarding", "Suporte Premium",
]
CANAIS = ["Orgânico", "Google Ads", "Meta Ads", "Indicação", "Parceiro", "Direto"]
CAMPANHAS = [
    "Brand sempre-on", "Performance Q3", "Retargeting carrinho",
    "Lançamento Scale", "Parceiros 2026", "ABM enterprise",
]
FORNECEDORES = ["NuvemAzul", "LogiSul", "Papel & Toner", "ChipNet", "Café Central", "Móveis Alfa"]
ARMAZENS = ["CD São Paulo", "CD Recife", "CD Curitiba"]
PLANOS = ["Start", "Pro", "Scale", "Enterprise"]

NOMES = [
    "Ana Costa", "Bruno Lima", "Carla Souza", "Diego Alves", "Eva Martins",
    "Fábio Nunes", "Gabriela Rocha", "Henrique Dias", "Isabela Freitas", "João Mendes",
    "Karina Lopes", "Lucas Azevedo", "Marina Teixeira", "Nicolas Barbosa", "Olívia Castro",
    "Paulo Ribeiro", "Queila Santos", "Rafael Pinto", "Sofia Carvalho", "Tiago Moreira",
    "Amanda Vieira", "Bernardo Cunha", "Catarina Melo", "Daniel Pires", "Elisa Duarte",
    "Felipe Andrade", "Giovana Reis", "Hugo Batista", "Júlia Fonseca", "Larissa Moura",
    "Mateus Peixoto", "Natália Borges", "Otávio Farias", "Patrícia Nogueira", "Renato Guimarães",
    "Sabrina Tavares", "Thales Prado", "Valentina Campos", "Alice Pacheco", "Helena Sales",
    "Igor Menezes", "Jéssica Brito", "Lívia Magalhães", "Murilo Coelho", "Nina Vasconcelos",
    "Priscila Guedes", "Stella Paiva", "Tainá Leal", "Vitória Sampaio", "Beatriz Furtado",
    "Caio Rezende", "Fernanda Assis", "Gustavo Cordeiro", "Letícia Aguiar", "Miguel Salles",
    "Nicole Braga", "Rafaela Quaresma", "Samuel Dutra", "Vinícius Porto", "Aline Macedo",
    "Camila Falcão", "Eduarda Mota", "Arthur Nascimento", "Débora Lins", "Enzo Pimentel",
    "Heloísa Bastos", "Joana Peres", "Kauã Trindade", "Pedro Henrique Luz", "Talita Rangel",
    "Diogo Saraiva", "Flávio Teles", "Benjamin Cruz", "Cecília Ramos", "Davi Figueiredo",
    "Kevin Siqueira", "Otto Lacerda", "Ruan Bezerra", "Ulisses Brandão", "William Barros",
    "Xavier Monteiro", "Yasmin Correia", "Zeca Antunes", "Wendy Araújo", "Úrsula Fernandes",
    "Vitor Campos", "Kaio Silveira", "Inês Cardoso", "Queila Fonseca", "Bruno César",
]


def money(n: float) -> str:
    return f"{n:.2f}"


def write_csv(path: Path, headers: list[str], rows: list[dict]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", newline="", encoding="utf-8") as f:
        w = csv.DictWriter(f, fieldnames=headers)
        w.writeheader()
        w.writerows(rows)
    print(f"  {path.name}: {len(rows)} linhas")


def month_starts(n: int = 12, end: date | None = None) -> list[date]:
    end = end or date(2026, 8, 1)
    out = []
    y, m = end.year, end.month
    for _ in range(n):
        out.append(date(y, m, 1))
        m -= 1
        if m == 0:
            m = 12
            y -= 1
    return list(reversed(out))


def random_day(start: date, end: date) -> date:
    span = (end - start).days
    return start + timedelta(days=random.randint(0, max(span, 0)))


def financeiro_dre() -> tuple[list[str], list[dict]]:
    linhas_receita = [
        ("Receita", "Receita bruta vendas", "Receita"),
        ("Receita", "Receita serviços", "Receita"),
        ("Receita", "Outras receitas", "Receita"),
    ]
    linhas_despesa = [
        ("Custos variáveis", "Custo do produto", "Despesa"),
        ("Custos variáveis", "Comissões", "Despesa"),
        ("Custos variáveis", "Impostos sobre vendas", "Despesa"),
        ("Custos fixos", "Folha", "Despesa"),
        ("Custos fixos", "Aluguel", "Despesa"),
        ("Custos fixos", "SaaS e ferramentas", "Despesa"),
        ("Custos fixos", "Marketing", "Despesa"),
        ("Custos fixos", "Informática e publicidade", "Despesa"),
    ]
    rows = []
    for dt in month_starts(12):
        growth = 1 + (dt.month + dt.year - 2025) * 0.015
        for empresa in EMPRESAS:
            for cat, linha, nat in linhas_receita:
                base = {"Receita bruta vendas": 42000, "Receita serviços": 18000, "Outras receitas": 2500}[linha]
                if empresa != "Redorai":
                    base *= 0.45 if empresa == "Redorai SP" else 0.28
                val = base * growth * random.uniform(0.92, 1.08)
                rows.append(row_fin(dt, cat, linha, nat, val, empresa))
            for cat, linha, nat in linhas_despesa:
                base = {
                    "Custo do produto": 14000, "Comissões": 4200, "Impostos sobre vendas": 5100,
                    "Folha": 22000, "Aluguel": 4800, "SaaS e ferramentas": 2100,
                    "Marketing": 3600, "Informática e publicidade": 900,
                }[linha]
                if empresa != "Redorai":
                    base *= 0.42 if empresa == "Redorai SP" else 0.25
                val = base * growth * random.uniform(0.9, 1.12)
                rows.append(row_fin(dt, cat, linha, nat, val, empresa))
    headers = ["data", "mes", "ano", "categoria", "linha", "natureza", "valor", "empresa"]
    return headers, rows


def row_fin(dt: date, cat: str, linha: str, nat: str, val: float, empresa: str) -> dict:
    return {
        "data": dt.isoformat(),
        "mes": MESES[dt.month],
        "ano": dt.year,
        "categoria": cat,
        "linha": linha,
        "natureza": nat,
        "valor": money(val),
        "empresa": empresa,
    }


def financeiro_inadimplencia() -> tuple[list[str], list[dict]]:
    statuses = [("Em dia", 0.55), ("Atraso 1-30", 0.22), ("Atraso 31-60", 0.12), ("Atraso 60+", 0.08), ("Negociação", 0.03)]
    rows = []
    start, end = date(2026, 1, 1), date(2026, 8, 28)
    for i, cliente in enumerate(CLIENTES):
        st = random.choices([s[0] for s in statuses], weights=[s[1] for s in statuses])[0]
        n_faturas = random.randint(2, 6)
        for _ in range(n_faturas):
            venc = random_day(start, end)
            valor = random.choice([890, 1490, 2890, 4900, 8900, 12400]) * random.uniform(0.8, 1.3)
            rows.append({
                "data": venc.isoformat(),
                "mes": MESES[venc.month],
                "ano": venc.year,
                "cliente": cliente,
                "status": st if random.random() > 0.15 else statuses[0][0],
                "valor": money(valor),
                "customer_count": 1,
                "empresa": random.choice(EMPRESAS),
            })
    headers = ["data", "mes", "ano", "cliente", "status", "valor", "customer_count", "empresa"]
    return headers, rows


def financeiro_orcamento() -> tuple[list[str], list[dict]]:
    linhas = [
        ("Receita", "Receita bruta", "Receita"),
        ("Receita", "Receita serviços", "Receita"),
        ("Pessoal", "Folha CLT", "Despesa"),
        ("Pessoal", "Benefícios", "Despesa"),
        ("Operação", "Infra e cloud", "Despesa"),
        ("Operação", "Aluguel", "Despesa"),
        ("Aquisição", "Mídia paga", "Despesa"),
        ("Aquisição", "Eventos", "Despesa"),
    ]
    rows = []
    for dt in month_starts(12):
        for cat, linha, nat in linhas:
            orcado = {
                "Receita bruta": 78000, "Receita serviços": 24000,
                "Folha CLT": 31000, "Benefícios": 4200,
                "Infra e cloud": 5400, "Aluguel": 4800,
                "Mídia paga": 9000, "Eventos": 1800,
            }[linha] * (1 + dt.month * 0.01)
            realizado = orcado * random.uniform(0.86, 1.18)
            if nat == "Receita" and dt.month in (1, 2):
                realizado *= 0.88
            rows.append({
                "data": dt.isoformat(),
                "mes": MESES[dt.month],
                "ano": dt.year,
                "categoria": cat,
                "linha": linha,
                "natureza": nat,
                "valor": money(realizado),
                "orcado": money(orcado),
                "empresa": "Redorai",
            })
    headers = ["data", "mes", "ano", "categoria", "linha", "natureza", "valor", "orcado", "empresa"]
    return headers, rows


def comercial_performance() -> tuple[list[str], list[dict]]:
    rows = []
    start, end = date(2026, 1, 2), date(2026, 8, 28)
    for _ in range(280):
        dt = random_day(start, end)
        produto = random.choice(PRODUTOS)
        preco = {"Plano Start": 490, "Plano Pro": 1290, "Plano Scale": 3490, "Add-on API": 390,
                 "Add-on SSO": 290, "Consultoria": 8900, "Onboarding": 2500, "Suporte Premium": 790}[produto]
        qty = 1 if "Plano" in produto or "Add-on" in produto else random.randint(1, 3)
        if dt.month in (1, 2):
            preco *= 0.9
        if dt.month >= 7:
            preco *= 1.08
        rows.append({
            "data": dt.isoformat(),
            "mes": MESES[dt.month],
            "ano": dt.year,
            "vendedor": random.choice(VENDEDORES),
            "regiao": random.choice(REGIOES),
            "produto": produto,
            "cliente": random.choice(CLIENTES),
            "canal": random.choice(CANAIS),
            "quantidade": qty,
            "valor": money(preco * qty * random.uniform(0.95, 1.05)),
            "customer_count": 1,
            "empresa": "Redorai",
        })
    headers = ["data", "mes", "ano", "vendedor", "regiao", "produto", "cliente", "canal", "quantidade", "valor", "customer_count", "empresa"]
    return headers, rows


def comercial_pipeline() -> tuple[list[str], list[dict]]:
    etapas = ["Lead", "Qualificado", "Proposta", "Negociação", "Ganho", "Perdido"]
    pesos = [18, 16, 14, 12, 10, 8]
    rows = []
    start, end = date(2026, 3, 1), date(2026, 8, 28)
    for i in range(120):
        dt = random_day(start, end)
        etapa = random.choices(etapas, weights=pesos)[0]
        valor = random.choice([4900, 8900, 14900, 24900, 42000, 78000]) * random.uniform(0.7, 1.2)
        rows.append({
            "data": dt.isoformat(),
            "mes": MESES[dt.month],
            "ano": dt.year,
            "vendedor": random.choice(VENDEDORES),
            "cliente": CLIENTES[i % len(CLIENTES)] + (f" {i}" if i >= len(CLIENTES) else ""),
            "produto": random.choice(PRODUTOS[:4]),
            "status": etapa,
            "quantidade": 1,
            "valor": money(valor),
            "customer_count": 1,
            "empresa": "Redorai",
        })
    headers = ["data", "mes", "ano", "vendedor", "cliente", "produto", "status", "quantidade", "valor", "customer_count", "empresa"]
    return headers, rows


def ecommerce() -> tuple[list[str], list[dict]]:
    produtos = [
        ("Camiseta logo", "Vestuário", 89),
        ("Moletom", "Vestuário", 189),
        ("Caneca", "Acessórios", 49),
        ("Caderno", "Acessórios", 39),
        ("Boné", "Vestuário", 69),
        ("Garrafa", "Acessórios", 79),
        ("Kit onboarding", "Kits", 249),
        ("Licença anual", "Digital", 990),
        ("Gift card 100", "Digital", 100),
        ("Mouse pad", "Acessórios", 45),
    ]
    canais = ["Site", "App", "Marketplace", "WhatsApp", "Loja física"]
    rows = []
    start, end = date(2026, 1, 5), date(2026, 8, 28)
    for _ in range(320):
        dt = random_day(start, end)
        nome, cat, preco = random.choice(produtos)
        qty = 1 + random.randint(0, 3)
        if dt.month == 5:
            qty += 1
        rows.append({
            "data": dt.isoformat(),
            "mes": MESES[dt.month],
            "ano": dt.year,
            "canal": random.choice(canais),
            "produto": nome,
            "categoria": cat,
            "regiao": random.choice(REGIOES),
            "quantidade": qty,
            "valor": money(preco * qty * random.uniform(0.9, 1.05)),
            "empresa": "Redorai",
        })
    headers = ["data", "mes", "ano", "canal", "produto", "categoria", "regiao", "quantidade", "valor", "empresa"]
    return headers, rows


def rh() -> tuple[list[str], list[dict]]:
    papeis = [
        ("Comercial", "SDR", "CLT", 4200, 8),
        ("Comercial", "Account Executive", "CLT", 8500, 10),
        ("Comercial", "Gerente comercial", "CLT", 14500, 2),
        ("Engenharia", "Dev Júnior", "CLT", 6500, 6),
        ("Engenharia", "Dev Pleno", "CLT", 9500, 8),
        ("Engenharia", "Dev Sénior", "CLT", 14000, 5),
        ("Engenharia", "Tech Lead", "CLT", 18000, 2),
        ("Produto", "Product Manager", "CLT", 12500, 3),
        ("Produto", "Designer de produto", "CLT", 8200, 2),
        ("Marketing", "Analista de marketing", "CLT", 5500, 3),
        ("Marketing", "Coordenador de marketing", "CLT", 8800, 1),
        ("Financeiro", "Analista financeiro", "CLT", 6200, 3),
        ("Financeiro", "Controller", "CLT", 12500, 1),
        ("RH", "Analista de RH", "CLT", 5800, 2),
        ("RH", "Business Partner", "CLT", 11000, 1),
        ("Operações", "Analista de operações", "CLT", 5500, 3),
        ("Customer Success", "CSM", "CLT", 7200, 4),
        ("Customer Success", "Analista de CS", "CLT", 4800, 4),
        ("Customer Success", "Estagiário", "Estágio", 1800, 3),
    ]
    pool = []
    for dept, cargo, tipo, base, n in papeis:
        pool.extend([(dept, cargo, tipo, base)] * n)
    rows = []
    for i, (dept, cargo, tipo, base) in enumerate(pool):
        nome = NOMES[i % len(NOMES)]
        if i >= len(NOMES):
            nome = f"{nome} {i}"
        year = random.choices([2024, 2025, 2026], weights=[2, 5, 4])[0]
        month = random.randint(1, 12 if year < 2026 else 8)
        day = random.randint(1, monthrange(year, month)[1])
        dt = date(year, month, day)
        salario = round(base * random.uniform(0.92, 1.16) / 50) * 50
        r = random.random()
        status = "Afastado" if r < 0.04 else ("Desligado" if r < 0.09 else "Ativo")
        if cargo == "Estagiário":
            tipo = "Estágio"
        rows.append({
            "data": dt.isoformat(),
            "mes": MESES[dt.month],
            "ano": dt.year,
            "colaborador": nome,
            "departamento": dept,
            "cargo": cargo,
            "tipo_contrato": tipo,
            "status": status,
            "headcount": 1 if status != "Desligado" else 0,
            "salario": money(salario),
            "empresa": "Redorai",
        })
    rows.sort(key=lambda r: (r["data"], r["departamento"], r["colaborador"]))
    headers = ["data", "mes", "ano", "colaborador", "departamento", "cargo", "tipo_contrato", "status", "headcount", "salario", "empresa"]
    return headers, rows


def estoque() -> tuple[list[str], list[dict]]:
    itens = [
        ("Camiseta logo", "Vestuário"), ("Moletom", "Vestuário"), ("Caneca", "Acessórios"),
        ("Caderno", "Acessórios"), ("Boné", "Vestuário"), ("Garrafa", "Acessórios"),
        ("Kit onboarding", "Kits"), ("Mouse pad", "Acessórios"), ("Cabo USB-C", "Acessórios"),
        ("Suporte notebook", "Acessórios"), ("Cadeira", "Mobiliário"), ("Monitor 27", "Informática"),
    ]
    statuses = ["Disponível", "Baixo", "Ruptura", "Parado"]
    rows = []
    for dt in month_starts(8):
        for produto, cat in itens:
            for armazem in ARMAZENS:
                qty = random.randint(0, 180)
                if produto == "Monitor 27":
                    qty = random.randint(0, 12)
                if qty == 0:
                    st = "Ruptura"
                elif qty < 8:
                    st = "Baixo"
                elif qty > 140 and random.random() < 0.25:
                    st = "Parado"
                else:
                    st = "Disponível"
                rows.append({
                    "data": dt.isoformat(),
                    "mes": MESES[dt.month],
                    "ano": dt.year,
                    "produto": produto,
                    "categoria": cat,
                    "armazem": armazem,
                    "status": st,
                    "estoque": qty,
                    "quantidade": max(qty, 1),
                    "valor": money(qty * random.uniform(18, 220)),
                    "empresa": "Redorai",
                })
    headers = ["data", "mes", "ano", "produto", "categoria", "armazem", "status", "estoque", "quantidade", "valor", "empresa"]
    return headers, rows


def producao() -> tuple[list[str], list[dict]]:
    produtos = ["Kit onboarding", "Camiseta logo", "Moletom", "Caneca", "Caderno"]
    statuses = ["Concluído", "Em linha", "Retrabalho", "Parado"]
    rows = []
    start, end = date(2026, 6, 1), date(2026, 8, 28)
    d = start
    while d <= end:
        if d.weekday() < 5:
            for produto in produtos:
                qty = random.randint(20, 90)
                st = random.choices(statuses, weights=[70, 18, 8, 4])[0]
                if st == "Parado":
                    qty = random.randint(0, 8)
                rows.append({
                    "data": d.isoformat(),
                    "mes": MESES[d.month],
                    "ano": d.year,
                    "produto": produto,
                    "status": st,
                    "quantidade": qty,
                    "valor": money(qty * random.uniform(22, 85)),
                    "empresa": "Redorai",
                })
        d += timedelta(days=1)
    headers = ["data", "mes", "ano", "produto", "status", "quantidade", "valor", "empresa"]
    return headers, rows


def marketing() -> tuple[list[str], list[dict]]:
    rows = []
    for dt in month_starts(10):
        for campanha in CAMPANHAS:
            for canal in ["Google Ads", "Meta Ads", "LinkedIn", "Orgânico", "E-mail"]:
                if canal == "Orgânico" and campanha == "ABM enterprise":
                    continue
                sessoes = int(random.uniform(800, 18000) * (1.4 if canal == "Google Ads" else 1))
                conv = random.uniform(0.4, 4.8)
                receita = sessoes * (conv / 100) * random.uniform(90, 420)
                rows.append({
                    "data": dt.isoformat(),
                    "mes": MESES[dt.month],
                    "ano": dt.year,
                    "canal": canal,
                    "campanha": campanha,
                    "session_count": sessoes,
                    "conversao": round(conv, 2),
                    "valor": money(receita),
                    "quantidade": int(sessoes * conv / 100),
                    "empresa": "Redorai",
                })
    headers = ["data", "mes", "ano", "canal", "campanha", "session_count", "conversao", "valor", "quantidade", "empresa"]
    return headers, rows


def logistica() -> tuple[list[str], list[dict]]:
    statuses = ["Entregue", "Em trânsito", "Atraso", "Extravio"]
    transportadoras = ["LogiSul", "Rápido Nordeste", "Correios", "Jadlog", "Azul Cargo"]
    rows = []
    start, end = date(2026, 3, 1), date(2026, 8, 28)
    for i in range(220):
        dt = random_day(start, end)
        st = random.choices(statuses, weights=[72, 16, 10, 2])[0]
        frete = random.uniform(12, 180)
        if st == "Atraso":
            frete *= 1.25
        rows.append({
            "data": dt.isoformat(),
            "mes": MESES[dt.month],
            "ano": dt.year,
            "regiao": random.choice(REGIOES),
            "status": st,
            "fornecedor": random.choice(transportadoras),
            "cliente": random.choice(CLIENTES),
            "quantidade": 1,
            "frete": money(frete),
            "valor": money(frete),
            "empresa": "Redorai",
        })
    headers = ["data", "mes", "ano", "regiao", "status", "fornecedor", "cliente", "quantidade", "frete", "valor", "empresa"]
    return headers, rows


def saas() -> tuple[list[str], list[dict]]:
    rows = []
    precos = {"Start": 490, "Pro": 1290, "Scale": 3490, "Enterprise": 8900}
    for i, dt in enumerate(month_starts(14, date(2026, 8, 1))):
        growth = 1 + i * 0.06
        for plano in PLANOS:
            contas = int({"Start": 48, "Pro": 32, "Scale": 14, "Enterprise": 5}[plano] * growth)
            churn_n = max(0, int(contas * random.uniform(0.02, 0.09)))
            if plano == "Start":
                churn_n = max(1, int(contas * random.uniform(0.05, 0.12)))
            mrr = contas * precos[plano] * random.uniform(0.97, 1.03)
            for status, n, mrr_part in [
                ("Ativa", contas - churn_n, mrr * 0.92),
                ("Trial", max(2, contas // 8), mrr * 0.02),
                ("Cancelada", churn_n, mrr * 0.06),
            ]:
                rows.append({
                    "data": dt.isoformat(),
                    "mes": MESES[dt.month],
                    "ano": dt.year,
                    "categoria": plano,
                    "status": status,
                    "mrr": money(mrr_part),
                    "customer_count": n,
                    "churn_count": churn_n if status == "Cancelada" else 0,
                    "valor": money(mrr_part),
                    "empresa": "Redorai",
                })
    headers = ["data", "mes", "ano", "categoria", "status", "mrr", "customer_count", "churn_count", "valor", "empresa"]
    return headers, rows


def compras() -> tuple[list[str], list[dict]]:
    itens = [
        ("Instâncias cloud", "Infra", "NuvemAzul", 4200),
        ("Domínio e DNS", "Infra", "ChipNet", 180),
        ("Papel A4", "Escritório", "Papel & Toner", 95),
        ("Toner", "Escritório", "Papel & Toner", 340),
        ("Café", "Facilities", "Café Central", 220),
        ("Mesas", "Facilities", "Móveis Alfa", 890),
        ("Frete inbound", "Logística", "LogiSul", 560),
        ("Licenças design", "Software", "ChipNet", 1290),
    ]
    rows = []
    for dt in month_starts(10):
        for produto, cat, forn, base in itens:
            qty = random.randint(1, 6)
            cost = base * qty * random.uniform(0.92, 1.22)
            rows.append({
                "data": dt.isoformat(),
                "mes": MESES[dt.month],
                "ano": dt.year,
                "fornecedor": forn,
                "categoria": cat,
                "produto": produto,
                "quantidade": qty,
                "valor": money(cost),
                "empresa": "Redorai",
            })
    headers = ["data", "mes", "ano", "fornecedor", "categoria", "produto", "quantidade", "valor", "empresa"]
    return headers, rows


def atendimento() -> tuple[list[str], list[dict]]:
    statuses = ["Aberto", "Pendente", "Em andamento", "Resolvido", "Escalado"]
    canais = ["E-mail", "Chat", "WhatsApp", "Telefone", "Portal"]
    rows = []
    start, end = date(2026, 5, 1), date(2026, 8, 28)
    d = start
    while d <= end:
        n = random.randint(4, 14) if d.weekday() < 5 else random.randint(1, 4)
        for _ in range(n):
            st = random.choices(statuses, weights=[12, 10, 18, 52, 8])[0]
            rows.append({
                "data": d.isoformat(),
                "mes": MESES[d.month],
                "ano": d.year,
                "canal": random.choice(canais),
                "status": st,
                "cliente": random.choice(CLIENTES),
                "quantidade": 1,
                "valor": money(random.uniform(15, 220)),
                "customer_count": 1,
                "empresa": "Redorai",
            })
        d += timedelta(days=1)
    headers = ["data", "mes", "ano", "canal", "status", "cliente", "quantidade", "valor", "customer_count", "empresa"]
    return headers, rows


DATASETS = [
    ("redorai-financeiro-dre.csv", "P&L sob controlo", financeiro_dre),
    ("redorai-financeiro-inadimplencia.csv", "Inadimplência sob controlo", financeiro_inadimplencia),
    ("redorai-financeiro-orcamento.csv", "Orçamento vs realizado", financeiro_orcamento),
    ("redorai-comercial.csv", "Performance comercial", comercial_performance),
    ("redorai-pipeline.csv", "Pipeline e follow-up", comercial_pipeline),
    ("redorai-ecommerce.csv", "E-commerce", ecommerce),
    ("redorai-rh.csv", "RH e pessoas", rh),
    ("redorai-estoque.csv", "Ruptura zero", estoque),
    ("redorai-producao.csv", "Performance de produção", producao),
    ("redorai-marketing.csv", "Aquisição e campanhas", marketing),
    ("redorai-logistica.csv", "Logística e frete", logistica),
    ("redorai-saas.csv", "SaaS e recorrência", saas),
    ("redorai-compras.csv", "Compras e fornecedores", compras),
    ("redorai-atendimento.csv", "Atendimento e CS", atendimento),
]


def main() -> None:
    here = Path(__file__).resolve().parent
    downloads = Path.home() / "Downloads"
    print("A gerar mocks Redorai…")
    for name, _label, fn in DATASETS:
        headers, rows = fn()
        write_csv(here / name, headers, rows)
        write_csv(downloads / name, headers, rows)
    readme = here / "README.md"
    lines = [
        "# Mocks Redorai para a Loja",
        "",
        "Carregue o CSV em **Dados** e ative o painel correspondente em **Loja**.",
        "",
        "| Ficheiro | Painel na Loja |",
        "| --- | --- |",
    ]
    for name, label, _fn in DATASETS:
        lines.append(f"| `{name}` | {label} |")
    readme.write_text("\n".join(lines) + "\n", encoding="utf-8")
    print(f"README -> {readme}")


if __name__ == "__main__":
    main()
