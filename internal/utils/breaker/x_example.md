## Circuit Breaker

### Architecture
```text
                 ┌─────────────┐
                 │   CLOSED    │
                 │ (normal)    │
                 └─────┬───────┘
                       │
             jika gagal melebihi threshold
                       │
                       ▼
                 ┌─────────────┐
                 │    OPEN     │
                 │ (fail fast) │
                 └─────┬───────┘
                       │
            setelah timeout tercapai
                       │
                       ▼
                 ┌─────────────┐
                 │  HALF-OPEN  │
                 │ (uji coba)  │
                 └─────┬───────┘
          ┌────────────┴────────────┐
          │                         │
   jika sukses semua         jika gagal
          │                         │
          ▼                         ▼
   ┌─────────────┐          ┌─────────────┐
   │   CLOSED    │          │    OPEN     │
   │ (reset)     │          │ (block)     │
   └─────────────┘          └─────────────┘
```
### Flow
```text
[Service Call] → [CircuitBreaker.Execute()]
          │
          ▼
   ┌─────────────┐
   │ Check State │
   └──────┬──────┘
          │
 ┌────────┼────────┐
 │        │        │
 ▼        ▼        ▼
Closed   Open   Half-Open
 │        │        │
 │        │        └─> Jika quota habis → langsung gagal
 │        │
 │        └─> Return error cepat ("circuit breaker is open")
 │
 └─> Jalankan fn(ctx) → sukses / gagal
          │
   ┌──────┴──────┐
   │ Update Count │
   └──────┬──────┘
          │
   ┌──────┴───────────────┐
   │                      │
Jika gagal banyak   Jika sukses
   │                      │
   ▼                      ▼
Trip → Open         Tetap Closed
```