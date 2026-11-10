// Landing page: the live loop, the key band, how it works, the film, the verdict key, the theme dial, install.
(function () {
  var VF = window.VF, $ = function (id) { return document.getElementById(id) };
  var calm = matchMedia('(prefers-reduced-motion: reduce)').matches, phone = matchMedia('(max-width: 820px)');
  document.head.appendChild(document.createElement('style')).textContent = VF.css;
  document.querySelectorAll('pre[data-f]').forEach(function (p) { p.innerHTML = VF.f[p.dataset.f] });

  // the Neovim half of the split: the real template's solve(), with the fix
  var NV = '<pre class="nv"><b> 30 </b><u>void</u> solve() {\n<b> 31 </b>    <u>int</u> w;\n<b> 32 </b>    cin &gt;&gt; w;\n<b> 33 </b>    YES(w % 2 == 0<em> &amp;&amp; w &gt; 2</em>);\n<b> 34 </b>}\n<b> 35 </b>\n<b> 36 </b><u>int</u> main() {\n<b> 37 </b>    ios::sync_with_stdio(<u>false</u>);\n<b> 38 </b>    cin.tie(<u>nullptr</u>);\n<b> 39 </b>    <u>int</u> T = 1;\n<b> 40 </b>    <u>while</u> (T--) solve();\n<b> 41 </b>    <u>return</u> 0;\n<b> 42 </b>}\n<span class="bar2"> main.cpp                         33,29 </span>:w</pre>';
  function frame(el, id, wipe) {
    el.innerHTML = '<pre>' + VF.f[id] + '</pre>' + (id === 'nfix' ? NV : '');
    if (phone.matches) el.scrollLeft = id === 'nfix' || id === '01' ? 0 : el.scrollWidth; // phones scroll: the tests pane is on the right
    if (wipe && !calm) { el.classList.remove('wipe'); void el.offsetWidth; el.classList.add('wipe') }
  }

  // --- headline: words rise one by one
  var h1 = document.querySelector('.hero h1');
  h1.innerHTML = h1.textContent.split(' ').map(function (w, n) { return '<span class="w"><i style="--n:' + n + '">' + w + '</i></span>' }).join(' ');

  // --- hero: real frames of 4A Watermelon, advanced by the real keys
  var S = [
    { f: ['05'], key: 't', cue: 'Press t to run the samples' },
    { f: ['09', '12'], key: 'S', cue: 'Both pass. S hunts a counterexample' },
    { f: ['16', '19'], key: 'w', cue: 'w = 2 breaks it. Save it as a test' },
    { f: ['20'], key: 'e', cue: 'e opens Neovim. Fix it' },
    { f: ['nfix'], key: 't', cue: 'Saved. Test again' },
    { f: ['24', '27'], key: 's', cue: 'Three of three. Submit' },
    { f: ['28', '30', '32'], key: '↵', cue: 'Accepted. Every step was one key' }
  ];
  var MOOD = { '19': 'wa', '20': 'wa', '32': 'ac' };
  var term = $('term'), now = $('now'), cue = $('cue'), stage = $('stage');
  var at = 0, timers = [], auto = null, typing = null;
  function paint(id, first) { frame(term, id, first); stage.className = 'stage ' + (MOOD[id] || '') }
  function type(text) {
    clearInterval(typing);
    if (calm) { cue.textContent = text; return }
    var n = 0; cue.textContent = '';
    typing = setInterval(function () { cue.textContent = text.slice(0, ++n); if (n >= text.length) clearInterval(typing) }, 22);
  }
  function go(i) {
    timers.forEach(clearTimeout); timers = [];
    at = (i + S.length) % S.length;
    var s = S[at];
    s.f.forEach(function (id, n) { n ? timers.push(setTimeout(paint, n * 640, id)) : paint(id, true) });
    now.textContent = s.key; type(s.cue);
  }
  function press(user) {
    if (user) clearInterval(auto);
    now.classList.add('down'); setTimeout(function () { now.classList.remove('down') }, 140);
    go(at + 1);
  }
  now.onclick = function () { press(true) };
  go(0);
  var heroSeen = true;
  if (!calm) auto = setInterval(function () { if (heroSeen && !document.hidden) press(false) }, 3400);
  // the rig leans a little toward the pointer
  var rig = $('rig');
  if (!calm && matchMedia('(hover: hover)').matches) {
    rig.onpointermove = function (e) {
      var r = rig.getBoundingClientRect();
      rig.style.setProperty('--ry', ((e.clientX - r.left) / r.width - .5) * 3 + 'deg');
      rig.style.setProperty('--rx', ((e.clientY - r.top) / r.height - .5) * -3 + 'deg');
    };
    rig.onpointerleave = function () { rig.style.setProperty('--ry', '0deg'); rig.style.setProperty('--rx', '0deg') };
  }

  // --- the band: verd's keys, and the one you press lights up
  var KEYS = [['t', 'run the tests'], ['e', 'edit in Neovim'], ['S', 'stress test'], ['w', 'save the counterexample'], ['s', 'submit'], ['H', 'editorial'], ['i', 'snippets'],
    ['d', 'diff a failure'], ['a', 'add a test'], ['T', 'manage tests'], ['/', 'search'], ['f', 'filters'], ['y', 'copy the statement'], ['m', 'mark solved'], ['?', 'help'], ['Z', 'zen mode']];
  var row = KEYS.map(function (k) { return '<span><b class="key" data-k="' + k[0] + '">' + k[0] + '</b>' + k[1] + '</span>' }).join('');
  $('band').firstElementChild.innerHTML = row + row;

  // --- how it works: the step in the middle of the screen picks the frame
  var chapters = [].slice.call($('chapters').children), howterm = $('howterm'), shown;
  function chapter(li) {
    if (li === shown) return; shown = li;
    chapters.forEach(function (c) { c.classList.toggle('on', c === li) });
    frame(howterm, li.dataset.f, true);
  }
  if (phone.matches) {
    chapters.forEach(function (li) { var d = document.createElement('div'); d.className = 'term'; d.setAttribute('data-ui', ''); li.appendChild(d); frame(d, li.dataset.f) });
  } else if ('IntersectionObserver' in window) {
    var cio = new IntersectionObserver(function (es) { es.forEach(function (e) { if (e.isIntersecting) chapter(e.target) }) }, { rootMargin: '-45% 0px -45% 0px' });
    chapters.forEach(function (li) { cio.observe(li) });
    chapter(chapters[0]);
  } else chapter(chapters[0]);

  // --- film
  var vid = $('vid'), player = $('player');
  $('play').onclick = function () { vid.controls = true; player.classList.add('playing'); vid.play() };

  // --- verdict: one key, two outcomes
  var sec = $('verdict'), vkey = $('vkey'), ac = false;
  function verdict() {
    ac = !ac; sec.classList.toggle('ac', ac);
    $('vread').innerHTML = ac ? '3/3 AC<br>3 ms, 3.4 MB' : 'custom-2 &nbsp;WA<br>want NO, got YES';
    $('vhint').innerHTML = ac ? 'Now it is worth a submission. Press <kbd>t</kbd> to see the failure again.' : 'Fix the w = 2 case, then press <kbd>t</kbd>.';
    vkey.classList.add('down'); setTimeout(function () { vkey.classList.remove('down') }, 140);
  }
  vkey.onclick = verdict;

  // --- themes: the real Settings frame in each theme
  var T = [['terminal', '#7aa7ff', '#0f1012', '#0f1012'], ['catppuccin', '#89b4fa', '#1e1e2e', '#1e1e2e'], ['dracula', '#bd93f9', '#282a36', '#282a36'],
    ['gruvbox', '#83a598', '#282828', '#282828'], ['nord', '#88c0d0', '#2e3440', '#2e3440'], ['one-dark', '#61afef', '#282c34', '#282c34'],
    ['solarized', '#268bd2', '#002b36', '#002b36'], ['tokyo-night', '#7aa2f7', '#1a1b26', '#1a1b26']];
  var themes = $('themes'), dial = $('tdial'), names = [].slice.call($('tnames').children);
  function theme() {
    var i = +dial.value, t = T[i];
    themes.style.setProperty('--tac', t[1]); themes.style.setProperty('--ton', t[2]); themes.style.setProperty('--tbg', t[3]);
    $('tname').textContent = t[0]; $('tframe').innerHTML = VF.f['set0' + i];
    names.forEach(function (li, n) { li.className = n === i ? 'on' : '' });
  }
  dial.oninput = theme; theme();
  names.forEach(function (li, n) { li.onclick = function () { dial.value = n; theme() } });

  // --- keys: the page answers to the same keys verd does
  var inView = {};
  if ('IntersectionObserver' in window) {
    var io = new IntersectionObserver(function (es) { es.forEach(function (e) { inView[e.target.id] = e.isIntersecting; if (e.target.id === 'top') heroSeen = e.isIntersecting }) }, { threshold: .35 });
    [$('top'), sec].forEach(function (el) { io.observe(el) });
    new IntersectionObserver(function (es, o) { es.forEach(function (e) { if (e.isIntersecting) { player.classList.add('in'); o.disconnect() } }) }, { threshold: .2 }).observe($('film')); // the player itself is clipped away, so it never intersects
  } else { inView.top = true; player.classList.add('in') }
  addEventListener('keydown', function (e) {
    if (e.metaKey || e.ctrlKey || e.altKey || /INPUT|TEXTAREA|SELECT/.test(e.target.tagName)) return;
    document.querySelectorAll('.band [data-k="' + (window.CSS && CSS.escape ? CSS.escape(e.key) : e.key) + '"]').forEach(function (k) { k.classList.add('lit'); setTimeout(function () { k.classList.remove('lit') }, 420) });
    if (inView.verdict && e.key === 't') return verdict();
    if (inView.top && (e.key === S[at].key || (at === S.length - 1 && e.key === 'Enter' && e.target === document.body))) press(true);
  });

  // --- install dialog
  var cmd = $('cmd'), copy = $('copy'), seg = [].slice.call(document.querySelectorAll('.seg button'));
  seg.forEach(function (b) { b.onclick = function () { seg.forEach(function (o) { o.setAttribute('aria-pressed', o === b) }); cmd.value = b.dataset.cmd } });
  cmd.onfocus = function () { cmd.select() };
  copy.onclick = function () {
    (navigator.clipboard ? navigator.clipboard.writeText(cmd.value) : Promise.reject()).then(
      function () { copy.textContent = 'Copied'; setTimeout(function () { copy.textContent = 'Copy command' }, 1500) },
      function () { cmd.focus(); cmd.select(); copy.textContent = 'Selected, press ctrl+c' });
  };
})();
