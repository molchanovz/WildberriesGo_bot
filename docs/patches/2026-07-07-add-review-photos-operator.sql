-- Review photos (customer-attached, shown to the operator in Telegram) and the
-- toOperator routing flag (persisted so the processing cron can route a review
-- to a human without re-running the model).
alter table reviews add column if not exists photos text[];
alter table reviews add column if not exists "toOperator" boolean not null default false;
