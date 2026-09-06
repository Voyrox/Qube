const copyButton = document.querySelector('.copy-command');
if (copyButton) {
  copyButton.addEventListener('click', async () => {
    const text = copyButton.dataset.copy;
    try {
      await navigator.clipboard.writeText(text);
      const label = copyButton.querySelector('.copy-label');
      const before = label.textContent;
      label.textContent = 'COPIED';
      setTimeout(() => label.textContent = before, 1200);
    } catch {
      // Clipboard may be unavailable in local file previews.
    }
  });
}

const terminal = document.querySelector('#terminal-demo');
const typed = terminal?.querySelector('.typed');
const caret = terminal?.querySelector('.caret');
const laterLines = terminal ? [...terminal.querySelectorAll('.terminal-line')] : [];
let played = false;

function runTerminal() {
  if (!typed || played) return;
  played = true;
  const text = typed.dataset.text || '';
  let i = 0;
  const tick = () => {
    typed.textContent = text.slice(0, i++);
    if (i <= text.length) {
      setTimeout(tick, 22 + Math.random() * 26);
      return;
    }
    if (caret) caret.style.display = 'none';
    laterLines.forEach((line, index) => {
      setTimeout(() => line.classList.add('show'), 260 + index * 320);
    });
  };
  tick();
}

if (terminal && 'IntersectionObserver' in window) {
  const observer = new IntersectionObserver((entries) => {
    if (entries.some(entry => entry.isIntersecting)) {
      runTerminal();
      observer.disconnect();
    }
  }, { threshold: .35 });
  observer.observe(terminal);
} else {
  runTerminal();
}
