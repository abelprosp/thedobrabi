-- Plano Grátis como predefinição para contas novas.
-- Organizações já existentes mantêm o plano actual.

ALTER TABLE organizations ALTER COLUMN plan SET DEFAULT 'gratis';
