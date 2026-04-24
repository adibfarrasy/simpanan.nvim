// MongoDB schema + seed data for the simpanan examples.
// Auto-runs on first container start because it lives in
// /docker-entrypoint-initdb.d/.

db = db.getSiblingDB("simp");

// ---- orders -----------------------------------------------------

db.orders.insertMany([
    {
        _id: "ord-001", user_id: "1", status: "paid", total: 120.00,
        placed_at: "2026-04-15T10:00:00Z",
        items: [
            { product_id: 1, qty: 2 },
            { product_id: 3, qty: 1 },
        ],
    },
    {
        _id: "ord-002", user_id: "1", status: "paid", total: 45.50,
        placed_at: "2026-04-18T13:45:00Z",
        items: [{ product_id: 2, qty: 1 }],
    },
    {
        _id: "ord-003", user_id: "2", status: "paid", total: 78.25,
        placed_at: "2026-04-08T09:15:00Z",
        items: [
            { product_id: 1, qty: 1 },
            { product_id: 5, qty: 1 },
        ],
    },
    {
        _id: "ord-004", user_id: "3", status: "paid", total: 210.00,
        placed_at: "2026-03-25T18:00:00Z",
        items: [
            { product_id: 4, qty: 2 },
            { product_id: 1, qty: 5 },
        ],
    },
    {
        _id: "ord-005", user_id: "3", status: "pending", total: 33.10,
        placed_at: "2026-04-19T22:00:00Z",
        items: [{ product_id: 2, qty: 1 }],
    },
    {
        _id: "ord-006", user_id: "4", status: "paid", total: 9.99,
        placed_at: "2026-04-20T07:30:00Z",
        items: [{ product_id: 3, qty: 1 }],
    },
]);

// ---- activity ---------------------------------------------------

db.activity.insertMany([
    { user_id: 1, event: "page_view",  ts: "2026-04-22T08:00:00Z" },
    { user_id: 1, event: "purchase",   ts: "2026-04-22T08:30:00Z" },
    { user_id: 2, event: "page_view",  ts: "2026-04-22T09:00:00Z" },
    { user_id: 3, event: "page_view",  ts: "2026-04-22T10:15:00Z" },
    { user_id: 3, event: "purchase",   ts: "2026-04-22T10:45:00Z" },
    { user_id: 4, event: "page_view",  ts: "2026-04-22T11:30:00Z" },
    { user_id: 5, event: "page_view",  ts: "2026-04-22T12:00:00Z" },
]);

print("simp: seeded", db.orders.countDocuments(), "orders and",
      db.activity.countDocuments(), "activity events.");
