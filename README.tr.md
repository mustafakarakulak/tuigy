# tuigy

*[English](README.md) · Türkçe*

Terminalden çıkmadan branch, commit, diff, merge ve cherry-pick yönetmek için hızlı,
klavye odaklı bir Git TUI'si.

tuigy, AI coding agent'larla birlikte çalışan geliştiriciler için tasarlandı. Agent'ı bir
terminal panelinde, tuigy'yi diğerinde çalıştırırsın: agent yazdıkça dosya listesi ve
diff'ler kendiliğinden tazelenir, böylece bir IDE'ye dokunmadan inceleyip stage'leyip
commit'leyebilirsin.

Bilinçli olarak **tam bir Git istemcisi değil**, ve dosya düzenlemiyor. Yaptığı şey,
döngünün geri kalanını tek ekranda tutmak: diff'in yanında repository ağacı, altında
testleri çalıştıracağın bir kabuk, ve çalıştığın her repository bir tuş uzakta.

## Durum

İlk plandaki her şey tamamlandı. Bugün çalışanlar:

- repository başlığı: branch, upstream, ahead/behind, kirli durum, yarım kalmış merge veya cherry-pick
- çakışma / staged / unstaged / untracked olarak ayrılmış değişiklikler görünümü; ortak
  öneki tekrarlamaya değdiğinde dizine göre gruplanır
- working tree, index ve takipsiz dosyalar için diff görüntüleyici: iki yönde kayar,
  değişen bir satırda gerçekten neyin değiştiğini kelime düzeyinde gösterir, hunk'lar
  arasında atlar
- stage, unstage, hepsini stage, hepsini unstage, değişikliği atma (onay sorarak), ve
  diff'in içinden tek bir hunk'ı stage/unstage etme
- commit ve amend
- branch listesi: her branch için upstream, ahead/behind ve son commit
- branch değiştirme, remote branch'i local'e alma, seçilen kaynaktan yeni branch, branch silme
- fetch, fetch all, pull ve push
- kaynak → hedef'i açıkça seçtiren merge; yerinde mi birleşeceğini, fast-forward mı
  olacağını yoksa hedef branch'i checkout mu edeceğini önceden söyler
- çakışma yönetimi: çakışan dosyalar listelenir, editöründe açılır, çözüldü olarak
  stage'lenir, sonra merge veya cherry-pick sürdürülür ya da iptal edilir
- herhangi bir branch'in commit geçmişi; her commit'in mesajı, dosyaları ve diff'i. `/`
  aramayı git üzerinden yapar, yani yüklü sayfadan daha eski bir eşleşme de bulunur
- cherry-pick: bir veya birden fazla commit seç, uygulanacak branch'i belirle; commit'ler
  yazıldıkları sırayla tekrar oynatılır
- stash: working tree'yi mesajla kenara al, her stash'in ne tuttuğunu incele, sonra pop,
  apply veya drop et
- AI commit mesajı: commit ekranında `ctrl+g`, staged diff'i zaten çalıştırdığın bir
  coding agent'a verir ve taslağını düzenlenebilir alana koyar
- canlı yenileme: başka bir işlemin yaptığı değişiklikler tuşa basmadan görünür
- gözden geçirme işaretleri: agent'ın değişikliklerini okurken dosyaları işaretle; dosya
  altından yeniden yazılırsa işaret kendiliğinden düşer
- `/` ile her listede filtreleme, `Y` ile yol, branch adı, commit hash'i veya stash ref'i kopyalama
- tema ve bütün kısayollar için ayar ekranı: istediğin tuşa basarak değiştiriyorsun,
  her değişiklik yapıldığı anda yapılandırma dosyasına yazılıyor
- files görünümü: git'in izlediği her şeyin ağacı, dosyanın kendisi yanındaki panelde;
  tek klasörü, bir klasörü tüm içeriğiyle ya da ağacın tamamını katlayabilirsin
- iki panelin altında bir bantta gömülü kabuk, repository kökünde açılır; böylece testin
  çıktısı ve konusu olan dosyalar aynı anda ekranda olur
- tuigy'nin açıldığı her repository üzerinde proje değiştirici; yeniden başlatmadan,
  yerinde geçiş
- değiştirebildiğin temalar ve kısayollar

## Kurulum

Linux ve macOS için hazır binary'ler her
[sürüme](https://github.com/mustafakarakulak/tuigy/releases) eklenir.

tuigy `git` binary'sini kullanır, bu yüzden git 2.23 veya üstü gerekir (`git switch` ve
`git restore` için). CI, test paketini hem git 2.30 hem güncel git ile, Linux ve macOS
üzerinde çalıştırır.

Windows desteklenmiyor: dosyayı editörde açmak ve commit mesajı üretmek POSIX kabuğundan
geçiyor, terminal bandı da bir POSIX pseudo-terminal.

Go 1.24.2 veya üstüyle:

```sh
go install github.com/mustafakarakulak/tuigy/cmd/tuigy@latest
```

Ya da klondan:

```sh
go build -o tuigy ./cmd/tuigy
```

`tuigy --version` hangi sürümü çalıştırdığını söyler; hata bildirirken eklemeye değer.

## Kullanım

`tuigy`'yi bir Git repository'sinin içinde herhangi bir yerde çalıştır.

| Tuş | İşlev |
| --- | --- |
| `1` `2` `3` `4` `5` | files / changes / branches / history / stashes sekmesi |
| `↑` `↓` / `k` `j` | gezin |
| `g` / `G` | başa / sona git |
| `ctrl+d` / `ctrl+u` | diff veya detay panelini kaydır |
| `←` `→` / `h` `l` | diff'i yana kaydır |
| `]` / `[` | sonraki / önceki hunk'a atla |
| `space` (diff'te) | imlecin üzerindeki hunk'ı stage/unstage et |
| `esc` | diyaloğu kapat, panelden çık, filtreyi temizle |
| `/` | bulunduğun listeyi filtrele |
| `Y` | imlecin üzerindekini kopyala (yol, branch, commit hash'i, stash) |
| `,` | ayarlar: tema, ve hangi tuşun ne yaptığı |
| `?` | yardım (kaydırılabilir) |
| `r` | yenile |
| `q` | çık |

Files sekmesi:

| Tuş | İşlev |
| --- | --- |
| `→` / `l` | imlecin üzerindeki klasörü aç |
| `←` / `h` | klasörü kapat, ya da içinde bulunduğun klasöre çık |
| `+` / `-` | o klasörü ve içindeki her şeyi aç / kapat |
| `L` / `H` | ağacın tamamını aç / kapat |
| `enter` | klasörü aç, ya da dosyayı yanındaki panelde oku |
| `e` | dosyayı editöründe aç |

Changes sekmesi:

| Tuş | İşlev |
| --- | --- |
| `tab` | dosya listesi ile diff arasında geçiş |
| `enter` | diff'e odaklan |
| `space` | seçili dosyayı stage/unstage et |
| `s` / `u` | stage / unstage |
| `a` / `A` | hepsini stage / hepsini unstage |
| `d` | değişiklikleri at (önce sorar) |
| `v` | dosyayı gözden geçirildi olarak işaretle |
| `e` | seçili dosyayı editöründe aç |
| `S` | değişikliklerini stash'le |
| `c` / `C` | commit / son commit'i düzelt |
| `ctrl+g` | commit'le, mesajı coding agent yazsın |
| `ctrl+s` | commit mesajını onayla |

Branches sekmesi:

| Tuş | İşlev |
| --- | --- |
| `enter` | seçili branch'e geç |
| `n` | seçili branch'ten yeni branch oluştur |
| `m` | seçili branch'i belirlediğin bir branch'e merge et |
| `l` | o branch'in geçmişini göster |
| `D` | seçili local branch'i sil (önce sorar) |

History sekmesi:

| Tuş | İşlev |
| --- | --- |
| `enter` | commit detayına odaklan |
| `space` | commit'i cherry-pick için seç ve bir alta in |
| `y` | seçimi belirlediğin bir branch'e cherry-pick et |

Stashes sekmesi:

| Tuş | İşlev |
| --- | --- |
| `enter` | stash'i pop et |
| `a` | uygula, stash'i koruyarak |
| `D` | sil (önce sorar) |

Her yerde:

| Tuş | İşlev |
| --- | --- |
| `f` / `F` | fetch / tüm remote'ları fetch |
| `p` / `P` | pull / push |
| `m` | yarım kalmış merge veya cherry-pick varken: sürdür ya da iptal et |
| `t` | terminal bandını aç, ya da ona geri dön |
| `ctrl+o` | terminalden çık — kabuğa hiç ulaşmayan tek tuş |
| `T` | terminal bandını kapat |
| `ctrl+p` | başka bir repository'ye geç |

Çakışan bir dosyayı stage'lemek onu çözüldü olarak işaretler — `git add` ile aynı şey.

## Yapılandırma

tuigy hiçbir yapılandırma olmadan çalışır.

İçeriden değiştirmeye değer iki şey için `,` tuşuna bas:

- **Tema.** Listede gezerken bütün görünüm yeniden boyanır, çünkü bir renk şemasını
  kimse adına bakarak değerlendiremez. `enter` seçimi kalıcılaştırır.
- **Kısayollar.** Sayfaya `tab` ile geçilir. 63 eylemin hepsini listeler — `/` listeyi
  daraltır — ve `enter`, imlecin üzerindeki eyleme vermek istediğin tuşa basmanı
  bekler. `r` varsayılanı geri koyar. Her değişiklik anında geçerli olur ve yaptığın
  anda yapılandırma dosyana yazılır.

Başka bir yerde kullanılan bir tuş reddedilmez, sadece söylenir: `enter` hem branch'e
geçer, hem stash pop'lar, hem diyalog onaylar; bunların ikisi aynı anda ekranda olmaz.
İki tuş senin verebileceğin tuşlar değil — tuş soran ekrandan çıkış yolu olan `esc`, ve
her zaman çıkışı yapan `ctrl+c`.

Editör, tek tek renkler ve commit mesajı agent'ı için:

```sh
tuigy --init-config   # yorumlarla açıklanmış bir yapılandırma dosyası yaz
tuigy --config        # dosyanın yerini yazdır
tuigy --themes        # hazır temaları listele
tuigy --actions       # tuşa bağlanabilecek her eylemi listele
```

Dosya `~/.config/tuigy/config.yml` ya da `$XDG_CONFIG_HOME/tuigy/config.yml`:

```yaml
# Şunlardan biri: default, dracula, gruvbox, nord. Denemek için tuigy içinde "," tuşuna bas.
theme: dracula

# Bunların herhangi biri temanın üzerine, hex değer olarak ezilebilir.
colors:
  accent: "#bd93f9"   # mevcut branch, odaklı panel, seçili satır
  added: "#50fa7b"    # eklenen satırlar, yeni dosyalar, ileride olan branch
  removed: "#ff5555"  # silinen satırlar, silinen dosyalar, hatalar
  warning: "#ffb86c"  # kirli working tree, geride olan branch, yarım kalmış merge
  text: "#f8f8f2"
  muted: "#6272a4"    # etiketler, ipuçları, üstveri
  border: "#44475a"
  inverse: "#282a36"  # accent, removed veya warning üzerine yazılan metin

# "e" tuşunun ne açacağı. git'in kendi ayarını ezer. Grafik arayüzlü bir editöre
# beklemesini sağlayan bayrağı vermek gerekir, yoksa tuigy dosya hâlâ açıkken devam eder.
editor: "code --wait"

# Herhangi bir eylemi yeniden bağla. `tuigy --actions` bütün adları listeler.
# Tek tuş ya da tuş listesi; "space" boşluk tuşu demektir.
keys:
  commit: [c, ctrl+k]
  quit: Q
  stash-push: w

ai:
  command: my-agent --headless "write a conventional commit message"
```

Anlaşılamayan bir yapılandırma dosyası, tuigy'yi hangi dosyada neyin yanlış olduğunu
söyleyerek durdurur; başlayıp sessizce yok saymaz. Ayarlar ekranından tema kaydetmek
yalnızca o tek satırı düzenler, dolayısıyla yorumların ve yazdığın diğer her şey yerinde
kalır.

Yanında bir dosya daha durur: `ctrl+p`'nin sunduğu liste olan `projects.yml`. Onu tuigy
kendi yazar — tuigy'yi açtığın her repository'nin yolunu ve adını tutar, içerikleri
hakkında hiçbir şey tutmaz. İçinde elle düzenlemeye değer bir şey yok, silmek de
sıralamadan başka bir şey kaybettirmez.

**Font tuigy'nin ayarlayabileceği bir şey değil.** Terminal uygulaması karakter yazar;
onları hangi yazı tipinin çizeceği terminal emülatörüne aittir. tuigy'nin seçtiği şey
kullandığı bir avuç sembol — oklar, madde imleri, kutu çizgileri — ve bunlar makul Unicode
kapsamı olan bir font ister.

## Dosyanın bir kısmını stage'lemek

Diff odaktayken `]` ve `[` hunk'lar arasında gezer, `space` imlecin üzerindekini tek
başına karşıya geçirir. Dosya bundan sonra hem staged hem unstaged listesinde görünür —
ki gerçekten başına gelen budur.

Bir coding agent'ın yanında bu işe yarar: agent nadiren her değişikliği aynı commit'e ait
olan bir dosya üretir. Takipsiz dosyalar istisna — henüz patch uygulanacak bir taraf
olmadığı için bütün olarak stage'lenirler.

## Agent'ın yazdıklarını gözden geçirmek

Bir dosyayı okundu olarak işaretlemek için `v` tuşuna bas. İşaret listede görünür, başlık
ne kadarını bitirdiğini sayar, commit ekranı da staged olup da bakılmamış bir şey varsa
söyler.

Dosya değişirse işaret düşer. Olayın özü bu: senin okuduğun bir dosyayı yeniden yazan bir
agent, o okumayı geçersiz kılmıştır — ve durum yoklaması bunu kendi başına göremez, dosya
hâlâ sadece "değişmiş" görünür.

Bu bir kapı değil, kendine bıraktığın not. Hiçbir şey commit'i reddetmez.

## Sadece diff'i değil, repository'yi de okumak

Files sekmesi git'in izlediklerinin ağacıdır: bütün takip edilen dosyalar, artı
`.gitignore`'un dışlamadığı takipsizler. Derleme çıktısı, `node_modules` ve vendor'lanmış
bağımlılıklar hiç görünmez, çünkü repository'de de yoklar.

Ağaç, değişmiş her dosyaya giden yol açılmış hâlde gelir; içinde değişiklik olan kapalı
bir klasör de işaretlenir, yani iş nerede olduğu hiçbir şey açmadan görünür. `+` ve `-`
bir alt ağacın tamamını alır ya da kaldırır; `L` ve `H` aynısını ağacın tümüne yapar.

Yanındaki panel dosyayı satır numaralarıyla gösterir — bir stack trace'i karşısında
okuduğun şey. Salt okunurdur: `e` dosyayı yine editörüne devreder. tuigy'de hiçbir şey
dosya içeriğine yazmaz.

## Çıkmadan bir şey çalıştırmak

`t` en altta, repository kökünde bir kabuk açar. İki panelin yerine geçmez, altlarına
oturur; böylece testin çıktısı ile konusu olan dosyalar aynı anda ekranda olur.

Klavye ondayken **her tuş kabuğa gider**, `ctrl+c` dahil — kesilemeyen bir kabuk kabuk
değildir. Ayrılmış tek tuş var: `ctrl+o` klavyeyi tuigy'ye geri verir, ve kabuk
klavyedeyken altbilgi başka hiçbir şey göstermez. `t` bıraktığın işe geri döner, `T`
bandı kapatır; `exit` yazmak da kapatır, çıkarken repository yeniden okunur.

Gerçek bir terminaldir, bir kayıt defteri değil: `vim`, bir test watcher'ı ve bir agent
doğru çizilir, pencereyi yeniden boyutlandırmak içindekini de boyutlandırır. Yukarı kayıp
giden şey çalışan programın kendi sorunudur — bandın kendine ait bir scrollback'i yok.

## Repository'ler arasında gezinmek

`ctrl+p`, tuigy'nin açıldığı her repository'yi en yeniden eskiye listeler. Daraltmak için
yaz, geçmek için `enter`, listeden düşürmek için `D`.

Projeyi kaydeden bir adım yok: tuigy'yi bir yerde açmak onu kaydeden şeydir. Geçiş,
yeniden başlatmak yerine çalışan süreçteki repository'yi değiştirir; böylece teman,
kısayolların ve editörün korunur, eski repository'yi tarif eden her şey — imleç, filtre,
gözden geçirme işaretleri — ekranda olmayan bir şeyi tarif etmesin diye bırakılır.

## `e` tuşu hangi editörü açar

Sırasıyla: tuigy'nin kendi `editor:` ayarı, sonra git'in kullanacağı şey
(`GIT_EDITOR`, `core.editor`, `VISUAL`, `EDITOR`), ve ancak ondan sonra bir varsayılan.

İlginç olan son adım. git'in kendi son çaresi `vi`'dir; hiç seçmediysen düşmek için
şaşırtıcı bir yerdir. Bu yüzden tuigy önce daha dostça bir terminal editörü arar —
`micro`, sonra `nano` — ve `vi`'yi ancak başka hiçbir şey yoksa kullanır. Gerçekten
ayarladığın ne ise ona her zaman saygı duyulur, `vi` dahil.

Tuşun kendi ipucu editörü adıyla söyler; alt bar `e open in nano` diye yazar, basıp
öğrenmene gerek kalmaz.

```yaml
editor: "code --wait"   # ya da: cursor --wait, zed --wait, nvim, hx, …
```

Grafik arayüzlü bir editör, dosyanın kapanmasını beklemesini sağlayan bayrağı ister.
Onsuz tuigy hemen devam eder; değişiklik yine de görünür, çünkü görünüm kendiliğinden
tazelenir, ama hiçbir şey senin için duraklamaz.

## Coding agent'ın yazdığı commit mesajları

Commit'lemek istediğini stage'le, sonra `ctrl+g`'ye bas. Commit ekranını açar, staged
diff'i bir coding agent'a verir ve yazdığı mesajı düzenlenebilir alana koyar. `ctrl+g`
tekrar başka bir mesaj ister; `ctrl+s` commit'ler; `esc` atar.

Sen onaylamadan hiçbir şey commit'lenmez, yani mesaj okuyup düzenlediğin bir taslaktır —
zaten yazacağın şeyin senin için yazılmış hâli.

tuigy bir model sağlayıcısıyla kendisi konuşmaz: API anahtarı yok, faturalandırma yok,
takip edilecek SDK yok. Zaten sende olan bir komutu çalıştırır. `claude` `PATH`'inde ise
otomatik kullanılır, ve tuş ipucu hangi agent'ın cevap vereceğini adıyla yazar; alt bar
`ctrl+g write with claude` der. Başka her şey yapılandırma dosyasına ya da önceliği olan
ortam değişkenine yazılır:

```sh
export TUIGY_AI_COMMIT='my-agent --headless "write a commit message"'
```

Komuta diff standart girdiden verilir ve mesajı yazdırması beklenir. Hiçbir agent
bulunamazsa tuş hiç teklif edilmez.

## Tasarım notları

**tuigy bir Git kütüphanesi kullanmak yerine `git` binary'sine shell-out eder.** Mevcut
yapılandırman, credential helper'ların, SSH agent'ın, hook'ların ve imzalama ayarların
hiçbir ek iş olmadan çalışır. Okumalar `GIT_OPTIONAL_LOCKS=0` ile yapılır, böylece
yoklama aynı repository'de çalışan bir coding agent'ı hiç bloklamaz; yazmalar sıraya
alınır ve `index.lock` çakışmasında yeniden denenir.

tuigy git'i sürerken git'in kendisi asla pager, editör veya terminal istemi açamaz. Yavaş
olan her şey — fetch, pull, kötü bağlantıda push — çalışırken ne yaptığını söyler, hem de
sonucun belireceği yerde. Credential helper'ı olmayan bir HTTPS remote'a push, arayüzü
kilitlemek yerine görünür bir hatayla hızlıca başarısız olur.

**Senin adına sürpriz bir şey olmaz.** `pull` yalnızca fast-forward yapar; ıraksamış bir
branch, istemediğin bir merge commit'iyle sessizce çözülmek yerine bildirilir. Local
değişiklikler branch geçişini engellediğinde tuigy stash'lemeyi teklif eder ve bunu
söyler; kendi başına asla stash'lemez. Üzerinde olmadığın bir branch'e merge, doğrudan
ref'e mi yazılabileceğini yoksa o branch'i checkout edip seni orada mı bırakacağını
önceden söyler.

## Tasarım ve kararlar

[docs/](docs) altında mimari notları ve karar kayıtları var: tuigy neden kütüphane
yerine git binary'sini çalıştırıyor, neden yoklama yapıyor, `pull` neden merge etmeyi
reddediyor ve bir git aracıyla bir editör arasındaki çizgi nerede. Bir şeyin çalışma
biçimini değiştirmeden ya da yeni bir şey önermeden önce okumaya değer; neyin tuigy'ye
ait olduğunu söyleyen kayıt
[15](docs/decisions/0015-a-workspace-around-the-git-tool.md). (Kayıtlar, kod gibi,
İngilizce.)

## Geliştirme

Buradaki hiçbir şey `make` gerektirmez — `go build ./...` ve `go test ./...` kendi başına
çalışır. Makefile bir kolaylık; tek başına `make` neler sunduğunu listeler:

```sh
make check       # biçim, vet ve testler: push'tan önce çalıştırılacak şey
make test        # test paketi
make race        # race detector ile testler
make cover       # paketler arası kapsam ve CI'ın dayattığı %90 eşiği
make cover-html  # ve boşlukların nerede olduğu
make linux       # testleri Linux'ta, CI'ın çalıştırdığı yerde koştur (Docker gerekir)
make snapshot    # yayınlamadan release artefaktlarını üret
make build       # ./tuigy
```

Testler gerçek `git` binary'sini geçici repository'ler üzerinde sürer, yani aracın
kullandığı kod yollarının taklidini değil kendisini çalıştırır. Kabuk veya dosya sistemiyle
ilgili her işten önce `make linux` çalıştırmaya değer: macOS ile CI'ın kullandığı container
arasında farklılaşan davranışları yakaladı.

CI aynı test paketini Linux ve macOS'ta, hem desteklenen en eski git hem güncel git ile
çalıştırır; kapsam eşiği için de `make cover` çağırır, böylece o sayı tek yerde tanımlıdır.
O job Go sürümünü sabitler: 1.27, kapsam bloklarının kaydedilme biçimini değiştirdi ve
aynı kod üzerindeki aynı testler 1.24'te %88.8, 1.27'de %91.4 raporluyor. Yani bir eşik,
ancak onu ölçen araç sürümüyle birlikte anlamlı.

## Lisans

MIT
