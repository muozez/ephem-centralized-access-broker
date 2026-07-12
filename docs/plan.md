# ephem — Centralized Ephemeral Access Broker

> [!NOTE]
> Bu plan ve mimari tasarım dondurulmuş (frozen) ve **[docs/architecture.md](file:///home/muozez/Documents/Github/ephem-centralized-access-broker/docs/architecture.md)** dosyasına taşınmıştır. Geliştirme sürecinde güncel mimari kararlar oradan takip edilecektir.

---

### v1.0 — Core MVP Başlangıç Adımları
Geliştirmeye başlarken takip edilecek temel patika:

1. **Phase 0 — Foundation**: `go mod init`, dizin yapısı, Go + PostgreSQL geliştirme compose ortamı.
2. **Phase 1 — Auth & CLI Skeleton**: OIDC login ve token yönetimi (`ephem login`).
3. **Phase 2 — Resource Registry & Policy Engine**: Glob/etiket tabanlı yetkilendirme motoru ve REST Admin API.
4. **Phase 3 — PostgreSQL Provider**: `IssueSession` ve `RevokeSession` implementasyonu.
5. **Phase 4 & 5 — Polling Scheduler & CLI Convenience**: Otomatik revoke polling mekanizması ve `ephem postgres <env>` alias'ları.
6. **Phase 6 — CLI Doctor**: `ephem doctor` komutu ile sistem sağlık testi.

Detaylı teknik çizimler, veri modeli ve Go arayüz tanımları için **[docs/architecture.md](file:///home/muozez/Documents/Github/ephem-centralized-access-broker/docs/architecture.md)** dosyasını inceleyebilirsiniz.
