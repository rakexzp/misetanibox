// Определение страны по имени сервера и путь к SVG-флагу (в /public/flags).
// Работает через: эмодзи-флаги (regional indicators), ISO-коды как слова, англ. и рус. названия.

const AVAILABLE = new Set([
  'us','gb','de','nl','fr','ru','ua','se','fi','no','dk','pl','ch','at','es','it','pt','tr',
  'jp','hk','sg','kr','tw','cn','in','au','ca','br','ae','il','cz','ro','lv','lt','ee','bg',
  'hu','sk','rs','md','kz','ge','am','az','by','th','vn','my','id','ph','sa','qa','bh','kw',
  'eg','za','mx','ar','cl','gr','ie','is','lu','be','si','hr','mk','al','me','xk','cy','mt',
  'na','uz','kg','tj','mn','np','pk','ir','iq','lk','bd',
]);

// текстовые подсказки (в нижнем регистре); ключ — ISO-2 код.
const KEYWORDS: Record<string, string[]> = {
  us: ['united states', 'usa', 'america', 'сша', 'америк'],
  gb: ['united kingdom', 'britain', 'england', 'uk', 'британ', 'англи', 'великобритан', 'лондон'],
  de: ['germany', 'герман', 'франкфурт', 'frankfurt'],
  nl: ['netherlands', 'holland', 'нидерланд', 'амстердам', 'amsterdam'],
  fr: ['france', 'франц', 'париж', 'paris'],
  ru: ['russia', 'росси', 'москв', 'moscow', 'спб', 'петербург'],
  ua: ['ukraine', 'украин', 'киев', 'kyiv', 'kiev'],
  se: ['sweden', 'швеци', 'стокгольм', 'stockholm'],
  fi: ['finland', 'финлянд', 'хельсинки', 'helsinki'],
  no: ['norway', 'норвег'],
  dk: ['denmark', 'дани'],
  pl: ['poland', 'польш', 'варшав', 'warsaw'],
  ch: ['switzerland', 'швейцар', 'цюрих', 'zurich'],
  at: ['austria', 'австри', 'вена', 'vienna'],
  es: ['spain', 'испани', 'мадрид', 'madrid'],
  it: ['italy', 'итали', 'милан', 'milan'],
  pt: ['portugal', 'португал'],
  tr: ['turkey', 'turkiye', 'турци', 'стамбул', 'istanbul'],
  jp: ['japan', 'япони', 'токио', 'tokyo'],
  hk: ['hong kong', 'hongkong', 'гонконг'],
  sg: ['singapore', 'сингапур'],
  kr: ['korea', 'корея', 'сеул', 'seoul'],
  tw: ['taiwan', 'тайван'],
  cn: ['china', 'китай'],
  in: ['india', 'инди'],
  au: ['australia', 'австрал', 'сидней', 'sydney'],
  ca: ['canada', 'канад'],
  br: ['brazil', 'бразил'],
  ae: ['emirates', 'dubai', 'оаэ', 'эмират', 'дубай'],
  il: ['israel', 'израил'],
  cz: ['czech', 'чехи', 'прага', 'prague'],
  ro: ['romania', 'румын'],
  lv: ['latvia', 'латви', 'рига', 'riga'],
  lt: ['lithuania', 'литв'],
  ee: ['estonia', 'эстони', 'таллин', 'tallinn'],
  bg: ['bulgaria', 'болгар'],
  hu: ['hungary', 'венгр'],
  sk: ['slovakia', 'словаки'],
  rs: ['serbia', 'серби'],
  md: ['moldova', 'молдов'],
  kz: ['kazakhstan', 'казахстан', 'алматы', 'almaty'],
  ge: ['georgia', 'грузи', 'тбилиси', 'tbilisi'],
  am: ['armenia', 'армени', 'ереван', 'yerevan'],
  az: ['azerbaijan', 'азербайдж'],
  by: ['belarus', 'беларус', 'минск', 'minsk'],
  th: ['thailand', 'таиланд', 'тайланд'],
  vn: ['vietnam', 'вьетнам'],
  my: ['malaysia', 'малайз'],
  id: ['indonesia', 'индонез'],
  ph: ['philippines', 'филиппин'],
  sa: ['saudi', 'саудов'],
  qa: ['qatar', 'катар'],
  bh: ['bahrain', 'бахрейн'],
  kw: ['kuwait', 'кувейт'],
  eg: ['egypt', 'египет'],
  za: ['south africa', 'юар'],
  mx: ['mexico', 'мексик'],
  ar: ['argentina', 'аргентин'],
  cl: ['chile', 'чили'],
  gr: ['greece', 'греци'],
  ie: ['ireland', 'ирланд'],
  is: ['iceland', 'исланд'],
  lu: ['luxembourg', 'люксембург'],
  be: ['belgium', 'бельги'],
  si: ['slovenia', 'словени'],
  hr: ['croatia', 'хорват'],
  cy: ['cyprus', 'кипр'],
  mt: ['malta', 'мальт'],
  uz: ['uzbekistan', 'узбекистан'],
  kg: ['kyrgyz', 'киргиз', 'кыргыз'],
  np: ['nepal', 'непал'],
  pk: ['pakistan', 'пакистан'],
  ir: ['iran', 'иран'],
  lk: ['lanka', 'ланка'],
  bd: ['bangladesh', 'бангладеш'],
};

const NAME_TO_CODE = new Set(AVAILABLE);

// эмодзи-флаг (пара regional indicators) → ISO-код
function codeFromEmoji(name: string): string | null {
  for (const ch of name) {
    const cp = ch.codePointAt(0) || 0;
    if (cp >= 0x1f1e6 && cp <= 0x1f1ff) {
      // нашли первый индикатор — собираем следующий
      const idx = Array.from(name).indexOf(ch);
      const arr = Array.from(name);
      const c1 = arr[idx]?.codePointAt(0) || 0;
      const c2 = arr[idx + 1]?.codePointAt(0) || 0;
      if (c1 >= 0x1f1e6 && c1 <= 0x1f1ff && c2 >= 0x1f1e6 && c2 <= 0x1f1ff) {
        const code = String.fromCharCode(97 + (c1 - 0x1f1e6)) + String.fromCharCode(97 + (c2 - 0x1f1e6));
        return code;
      }
    }
  }
  return null;
}

export function countryCode(name: string): string | null {
  if (!name) return null;
  const emoji = codeFromEmoji(name);
  if (emoji && NAME_TO_CODE.has(emoji)) return emoji;
  if (emoji === 'uk') return 'gb';

  const lower = name.toLowerCase();

  // текстовые названия (подстрока)
  for (const [code, words] of Object.entries(KEYWORDS)) {
    for (const w of words) {
      if (lower.includes(w)) return code;
    }
  }

  // ISO-код как отдельное «слово» (границы — не буквы)
  const tokens = lower.split(/[^a-z]+/);
  for (const t of tokens) {
    if (t === 'uk') return 'gb';
    if (t.length === 2 && NAME_TO_CODE.has(t)) return t;
  }
  return null;
}

export function flagUrl(name: string): string | null {
  const code = countryCode(name);
  return code && NAME_TO_CODE.has(code) ? `/flags/${code}.svg` : null;
}
