// Shared by every page: the bar, entrances, copy buttons, and "where am I" in the bar and the table of contents.
(function () {
  var calm = matchMedia('(prefers-reduced-motion: reduce)').matches, bar = document.getElementById('bar');
  function each(sel, fn) { [].forEach.call(document.querySelectorAll(sel), fn) }
  function clip(text, btn, done) {
    var old = btn.textContent;
    (navigator.clipboard ? navigator.clipboard.writeText(text) : Promise.reject()).then(
      function () { btn.textContent = 'Copied'; if (done) done(); setTimeout(function () { btn.textContent = old }, 1500) },
      function () { btn.textContent = 'Select + copy'; setTimeout(function () { btn.textContent = old }, 2000) });
  }

  // the bar steps aside on the way down, returns on the way up, and shows how far you are
  var last = 0, prog = bar.querySelector('.prog');
  addEventListener('scroll', function () {
    var y = scrollY, max = document.documentElement.scrollHeight - innerHeight;
    if (!bar.classList.contains('open')) bar.classList.toggle('away', y > last && y > 400);
    last = y; prog.style.setProperty('--p', max > 0 ? y / max : 0);
  }, { passive: true });
  var burger = bar.querySelector('.burger');
  function menu(open) { bar.classList.toggle('open', open); burger.setAttribute('aria-expanded', open); burger.textContent = open ? 'Close' : 'Menu'; document.body.style.overflow = open ? 'hidden' : '' }
  burger.onclick = function () { menu(!bar.classList.contains('open')) };
  each('.bar .links a', function (a, n) { a.style.setProperty('--n', n); a.addEventListener('click', function () { menu(false) }) });
  addEventListener('keydown', function (e) { if (e.key === 'Escape') menu(false) });

  // copy: any [data-copy] button, and every code block in the docs
  each('[data-copy]', function (b) {
    b.onclick = function () { var box = b.parentNode; clip(b.dataset.copy, b, function () { box.classList.add('ok'); setTimeout(function () { box.classList.remove('ok') }, 1500) }) };
  });
  each('.prose pre', function (p) {
    var b = document.createElement('button'); b.className = 'copy'; b.textContent = 'Copy';
    b.onclick = function () { clip(p.querySelector('code').innerText, b) }; p.appendChild(b);
  });

  if (!('IntersectionObserver' in window)) return;

  // entrances
  if (!calm) {
    var io = new IntersectionObserver(function (es) { es.forEach(function (e) { if (e.isIntersecting) { e.target.classList.add('in'); io.unobserve(e.target) } }) }, { rootMargin: '0px 0px -6% 0px' });
    each('.rv,.prose h2,.prose pre,.prose table,.docgrid a,.pager a,.lineup tbody tr,.mark span,.foot div', function (el, i) {
      if (!el.classList.contains('rv')) { el.classList.add('rv'); el.style.setProperty('--i', i % 5) }
      io.observe(el);
    });
  }

  // where am I: links that point at something on this page follow the scroll
  var spy = {};
  each('.bar .links a[href*="#"],.toc a', function (a) {
    var id = decodeURIComponent(a.hash.slice(1)), t = id && a.pathname === location.pathname && document.getElementById(id);
    if (t) spy[id] = { a: a, t: t };
  });
  var ids = Object.keys(spy);
  if (ids.length) {
    var so = new IntersectionObserver(function (es) {
      es.forEach(function (e) {
        if (!e.isIntersecting) return;
        ids.forEach(function (id) { spy[id].a.classList.toggle('on', spy[id].t === e.target) });
      });
    }, { rootMargin: '-20% 0px -70% 0px' });
    ids.forEach(function (id) { so.observe(spy[id].t) });
  }
})();
