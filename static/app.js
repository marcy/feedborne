(function () {
  var entries = [];
  var idx = 0;
  function collect() {
    var current = entries[idx];
    entries = Array.prototype.slice.call(document.querySelectorAll('.entry'));
    if (current && entries.indexOf(current) >= 0) {
      idx = entries.indexOf(current);
    }
    if (idx >= entries.length) idx = Math.max(0, entries.length - 1);
  }
  function select(i) {
    if (!entries.length) return;
    entries[idx].classList.remove('selected');
    idx = Math.max(0, Math.min(entries.length - 1, i));
    entries[idx].classList.add('selected');
    entries[idx].scrollIntoView({ block: 'nearest' });
  }
  collect();
  if (entries.length) entries[0].classList.add('selected');
  document.body.addEventListener('htmx:afterSwap', function () {
    var had = entries[idx];
    collect();
    entries.forEach(function (el) { el.classList.remove('selected'); });
    if (entries.length) {
      if (had && entries.indexOf(had) >= 0) idx = entries.indexOf(had);
      entries[idx].classList.add('selected');
    }
  });
  document.addEventListener('keydown', function (e) {
    if (e.target && (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA')) return;
    if (e.key === 'j') {
      select(idx + 1);
    } else if (e.key === 'k') {
      select(idx - 1);
    } else if (e.key === 'o') {
      var el = entries[idx];
      if (!el) return;
      window.open(el.getAttribute('data-url'), '_blank', 'noopener');
      el.classList.add('read');
      htmx.ajax('POST', '/read', {
        values: { id: el.getAttribute('data-id') },
        target: '#unread',
        swap: 'innerHTML'
      });
    }
  });
})();
