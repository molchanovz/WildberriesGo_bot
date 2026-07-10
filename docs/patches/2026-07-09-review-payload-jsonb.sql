-- Move review content (text/pros/cons/customerName/photos) into a single
-- payload jsonb column, together with WB-derived context we now feed the answer
-- generator (returnStatus, matchingSize, orderedAt, productName, subjectName,
-- color). New WB fields can be added to the payload with no further migration.
-- Shape: tradeplus.ReviewPayload.
--
-- Control-plane columns stay as columns (queried/filtered/status machine):
-- reviewId, cabinetId, externalId, article, valuation, statusId, answer,
-- toOperator, createdAt.

alter table reviews add column if not exists payload jsonb;

-- Backfill existing rows from the columns about to be dropped so nothing is lost.
update reviews
set payload = jsonb_strip_nulls(jsonb_build_object(
    'text', nullif(text, ''),
    'pros', nullif(pros, ''),
    'cons', nullif(cons, ''),
    'customerName', nullif("customerName", ''),
    'photos', case
        when photos is not null and array_length(photos, 1) > 0 then to_jsonb(photos)
        else null
    end
))
where payload is null;

alter table reviews drop column if exists text;
alter table reviews drop column if exists pros;
alter table reviews drop column if exists cons;
alter table reviews drop column if exists "customerName";
alter table reviews drop column if exists photos;
