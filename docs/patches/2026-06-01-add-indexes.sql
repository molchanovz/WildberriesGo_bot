create index "IX_FK_orders_cabinetId_orders"
    on public.orders ("cabinetId");

create index "IX_FK_products_cabinetId_products"
    on public.products ("cabinetId");

create index "IX_reviews_externalId"
    on public.reviews ("externalId");
