// Landing page: the live loop, the film, the verdict key, the theme dial, install.
(function () {
  var VF = window.VF, $ = function (id) { return document.getElementById(id) };
  var calm = matchMedia('(prefers-reduced-motion: reduce)').matches;
  document.head.appendChild(document.createElement('style')).textContent = VF.css;
  document.querySelectorAll('pre[data-f]').forEach(function (p) { p.innerHTML = VF.f[p.dataset.f] });

  // --- the loop: real frames of 4A Watermelon, advanced by the real keys
  var NV = '<pre class="nv"><b> 30 </b><u>void</u> solve() {\n<b> 31 </b>    <u>int</u> w;\n<b> 32 </b>    cin &gt;&gt; w;\n<b> 33 </b>    YES(w % 2 == 0<em> &amp;&amp; w &gt; 2</em>);\n<b> 34 </b>}\n<b> 35 </b>\n<b> 36 </b><u>int</u> main() {\n<b> 37 </b>    ios::sync_with_stdio(<u>false</u>);\n<b> 38 </b>    cin.tie(<u>nullptr</u>);\n<b> 39 </b>    <u>int</u> T = 1;\n<b> 40 </b>    <u>while</u> (T--) solve();\n<b> 41 </b>    <u>return</u> 0;\n<b> 42 </b>}\n<span class="bar"> main.cpp                         33,29 </span>:w</pre>';
  // each state: the frames it plays, then the key and cue that lead out of it
  var S = [
    { f: ['05'], key: 't', cue: 'Run the samples' },
    { f: ['09', '12'], key: 'S', cue: 'Both pass. Hunt a counterexample' },
    { f: ['16', '19'], key: 'w', cue: 'w = 2 breaks it. Save it as a test' },
    { f: ['20'], key: 'e', cue: 'Fix it in Neovim' },
    { f: ['nfix'], key: 't', cue: 'Test again' },
    { f: ['24', '27'], key: 's', cue: 'Three of three. Submit' },
    { f: ['28', '30', '32'], key: '\u21b5', cue: 'Accepted. Every step was one key' }
  ];
  var term = $('term'), now = $('now'), cue = $('cue'), steps = [].slice.call($('steps').children);
  var at = 0, timers = [], auto = null, touched = false;
  function paint(id) {
    term.innerHTML = '<pre>' + VF.f[id] + '</pre>' + (id === 'nfix' ? NV : '');
    if (term.scrollWidth > term.clientWidth) term.scrollLeft = id === 'nfix' ? 0 : term.scrollWidth; // phones: the tests pane is on the right
    if (!calm) { term.classList.remove('flash'); void term.offsetWidth; term.classList.add('flash') }
  }
  function go(i) {
    timers.forEach(clearTimeout); timers = [];
    at = (i + S.length) % S.length;
    var s = S[at];
    s.f.forEach(function (id, n) { n ? timers.push(setTimeout(paint, n * 620, id)) : paint(id) });
    now.textContent = s.key; cue.textContent = s.cue;
    steps.forEach(function (li, n) { li.className = n === at ? 'on' : n < at ? 'done' : '' });
  }
  function press(user) {
    if (user) { touched = true; clearInterval(auto) }
    now.classList.add('down'); setTimeout(function () { now.classList.remove('down') }, 130);
    go(at + 1);
  }
  now.onclick = function () { press(true) };
  steps.forEach(function (li, n) { li.firstElementChild.onclick = function () { touched = true; clearInterval(auto); go(n + 1) } });
  go(0);
  var heroSeen = true;
  if (!calm) auto = setInterval(function () { if (heroSeen && !document.hidden) press(false) }, 3000);

  // --- film
  var vid = $('vid'), player = $('player');
  $('play').onclick = function () { vid.controls = true; player.classList.add('playing'); vid.play() };

  // --- verdict: one key, two outcomes
  var sec = $('verdict'), vkey = $('vkey'), ac = false;
  function verdict() {
    ac = !ac; sec.classList.toggle('ac', ac);
    $('vword').textContent = ac ? 'accepted' : 'wrong answer';
    $('vread').innerHTML = ac ? '3/3 AC<br>3 ms, 3.4 MB' : 'custom-2 &nbsp;WA<br>want NO, got YES';
    $('vhint').innerHTML = ac ? 'Now it is worth a submission. Press <kbd>t</kbd> to see the failure again.' : 'Fix the w = 2 case, then press <kbd>t</kbd>.';
    vkey.classList.add('down'); setTimeout(function () { vkey.classList.remove('down') }, 130);
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
    if (inView.verdict && e.key === 't') return verdict();
    if (inView.top && (e.key === S[at].key || (at === S.length - 1 && e.key === 'Enter' && e.target === document.body))) press(true);
  });

  // --- install
  var cmd = $('cmd'), copy = $('copy'), seg = [].slice.call(document.querySelectorAll('.seg button'));
  seg.forEach(function (b) { b.onclick = function () { seg.forEach(function (o) { o.setAttribute('aria-pressed', o === b) }); cmd.value = b.dataset.cmd } });
  cmd.onfocus = function () { cmd.select() };
  copy.onclick = function () {
    (navigator.clipboard ? navigator.clipboard.writeText(cmd.value) : Promise.reject()).then(
      function () { copy.textContent = 'Copied'; setTimeout(function () { copy.textContent = 'Copy command' }, 1500) },
      function () { cmd.focus(); cmd.select(); copy.textContent = 'Selected, press ctrl+c' });
  };
})();
