# Ephem Centralized Access Broker — Doğrulama Kılavuzu

Bu kılavuz, **PostgreSQL (Dynamic Role Provisioning)** ve **SSH (CA Signed Certificates)** geçici erişim mekanizmalarını Docker Compose sandbox ortamında adım adım nasıl doğrulayacağınızı açıklar.

---

## Hazırlık ve Ortamın Başlatılması

### Adım 1: CLI İstemcisini Derleyin
Öncelikle yerel terminalinizde `ephem` komut satırı aracını (CLI) derleyin:
```bash
make build
```
Bu komut, `bin/ephem` CLI binary'sini oluşturacaktır.

### Adım 2: Docker Compose Sandbox Ortamını Başlatın
Tüm ekosistemi (Veritabanı, Merkezi API Sunucusu ve Hedef SSH Sunucusu) tek bir komutla başlatın:
```bash
docker compose up --build -d
```

### Adım 3: Konfigürasyon Verilerini Yükleyin (Seed SQL)
`postgres` veritabanı konteynerine yerel test kaynaklarını (PostgreSQL staging ve SSH staging hedefleri) ve erişim politikalarını yüklemek için şu komutu çalıştırın:
```bash
docker exec -i ephem-postgres psql -U ephem -d ephem -c "TRUNCATE sessions, audit_logs, policies, resources, provider_configs CASCADE;" && cat migrations/02_seed_resources.sql | docker exec -i ephem-postgres psql -U ephem -d ephem
```

### Adım 4: Konteynerlerin Başlangıç Uyumunu Kontrol Edin
Merkezi API sunucusunun başarıyla başladığını ve SSH CA anahtarlarını oluşturduğunu doğrulayın:
```bash
docker logs ephem-api
```
*Beklenen Çıktı:* `Listening on port 8080...` ve arka planda `ssh_ca.key` ile `ssh_ca.pub` dosyalarının oluşturulması.

Hedef SSH konteynerinin bu CA anahtarını algılayıp `sshd` servisini başlattığını doğrulayın:
```bash
docker logs ephem-target-ssh
```
*Beklenen Çıktı:*
```text
Waiting for SSH CA public key to be generated at /shared/ssh_ca.pub...
SSH CA public key detected. Copying to /etc/ssh/ca.pub...
Starting OpenSSH daemon...
Server listening on 0.0.0.0 port 22.
```

---

## Adım Adım Doğrulama Akışı

### Adım 5: CLI Üzerinden Giriş Yapın (Authentication)
CLI üzerinden kimlik doğrulama sürecini başlatın:
```bash
./bin/ephem login
```
1. Terminalde size bir link verilecektir. Tarayıcınızda bu linki açın.
2. Açılan Mock OIDC arayüzünde seed verilerinde kayıtlı olan `developer@company.com` mail adresini girin ve **Sign In** butonuna tıklayın.
3. Terminalde `Login successful!` mesajını göreceksiniz.
4. Kimliğinizi doğrulamak için şu komutları çalıştırın:
   ```bash
   ./bin/ephem whoami
   ./bin/ephem doctor
   ```
   *`ephem doctor` tüm bağımlılıklerin (JWKS, API ve psql istemcisi) sağlıklı olduğunu teyit etmelidir.*

---

### Adım 6: PostgreSQL Geçici Erişimini Test Edin
Geliştirici rolünüzle `postgres-staging` veritabanına bağlanmayı deneyin:
```bash
./bin/ephem postgres staging
```
**Arka Planda Neler Oluyor?**
1. CLI, API'ye bir JWT göndererek `postgres-staging` için oturum talep eder.
2. API'deki **Policy Engine**, `developer` rolünün bu kaynağa erişim iznini onaylar.
3. API, veritabanı üzerinde rastgele bir kullanıcı (`ephem_u_<session_id>`) ve şifre oluşturur.
4. Bu bilgileri CLI'a döner. **Not:** API sunucusu veritabanına Docker ağı içerisinden `host: postgres` adresiyle bağlanırken, istemciye dönen yanıttaki `client_host` parametresi `localhost` olarak belirlenir. Bu sayede yerel makinenizdeki CLI, port yönlendirmesi (port mapping) sayesinde `localhost:5432` üzerinden veritabanına sorunsuzca erişebilir.
5. CLI, bu bilgileri bellek ortam değişkenlerine (`PGUSER`, `PGPASSWORD`) atayıp yerel `psql` istemcinizi başlatır.
6. `psql` arayüzündeyken `SELECT current_user;` komutu ile geçici kullanıcınızı görebilirsiniz.
7. `\q` yazıp çıktığınız anda CLI, API'ye anında **Revocation** sinyali gönderir ve oluşturulan veritabanı kullanıcısı anında silinir (Zero Standing Privileges).

---

### Adım 7: SSH CA Sertifikalı Geçici Bağlantıyı Test Edin
Şimdi parola gerektirmeyen, tamamen SSH CA imzalı sertifika tabanlı SSH bağlantısını test edelim.

#### İnteraktif Olmayan Hızlı Komut Çalıştırma:
Hedef SSH sunucusunda geçici anahtarla `id` komutunu uzaktan çalıştırın:
```bash
./bin/ephem exec ssh-staging -- ssh id
```
*Beklenen Çıktı:*
```text
Requesting temporary access to resource 'ssh-staging'...
uid=1000(ubuntu) gid=1000(ubuntu) groups=1000(ubuntu)

Revoking ephemeral credentials...
Ephemeral credentials revoked successfully.
```

#### İnteraktif Kabuk (Shell) Bağlantısı:
Doğrudan hedef sunucunun terminaline bağlanmak için:
```bash
./bin/ephem ssh staging
```
**Arka Planda Neler Oluyor?**
1. CLI, API'ye `ssh-staging` erişim isteği gönderir.
2. API, bellekte geçici bir SSH anahtar çifti (Private/Public Key) üretir.
3. API, bu public key'i kendi güvenli **SSH CA Private Key**'i ile imzalar ve bir **SSH User Certificate** üretir (bu sertifikaya `ubuntu` kullanıcısı yetkisi ve 30 dakikalık geçerlilik süresi eklenir).
4. API, signed certificate ve private key verilerini CLI'a iletir. İstemcinin bağlanabilmesi için hedef bağlantı adresi `client_host: localhost` ve `client_port: 2222` olarak yapılandırılmıştır.
5. CLI, bu iki veriyi `~/.ephem/` dizini altına `ssh_temp_<session_id>` ve `ssh_temp_<session_id>-cert.pub` isimleriyle yazar (izinleri `0600` olarak kısıtlar).
6. CLI, alt süreç olarak `ssh` komutunu tetikler. Go alt süreçlerinde (subprocess) interaktif kabukların kilitlenmesini veya pseudo-terminal (PTY) hatası alınmasını engellemek için otomatik olarak `-t` parametresi eklenir (`ssh -t -i ~/.ephem/ssh_temp_<session_id> -p 2222 ubuntu@localhost`).
7. Hedef sunucu (`ephem-target-ssh`), gelen sertifikayı kendi içindeki `/etc/ssh/ca.pub` ile doğrular ve parola sormadan oturumu açar.
8. Terminalden `exit` yazıp çıktığınız anda CLI, diskteki geçici SSH anahtarlarını **tamamen siler** ve API'ye revoke bildirimi gönderir.

---

### Adım 8: Audit Loglarının Kontrol Edilmesi
Tüm bu işlemlerin veritabanında denetlendiğini (audited) doğrulamak için veritabanındaki log tablosunu sorgulayın:
```bash
docker exec -i ephem-postgres psql -U ephem -d ephem -c "SELECT id, action, resource_name, result FROM audit_logs ORDER BY id DESC LIMIT 5;"
```
*Beklenen Çıktı:* En üstte `request_session` ve `revoke_session` eylemlerinin `SUCCESS` sonucuyla loglandığını görmelisiniz.
