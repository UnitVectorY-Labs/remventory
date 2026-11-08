// The URL describes the current workspace; requests and HTMX status polling do
// not create history entries. Keep legacy ?identify links working as well.
let restoringView = false;

function updateViewURL(path, params = {}, replace = false) {
  const url = new URL(path, location.origin);
  for (const [key, value] of Object.entries(params)) {
    if (value !== '' && value !== undefined && value !== null) url.searchParams.set(key, value);
  }
  if (url.pathname + url.search === location.pathname + location.search) return;
  history[replace || restoringView ? 'replaceState' : 'pushState'](null, '', url);
}

function routePage(params) {
  const page = Number(params.get('page') || 1);
  return Number.isSafeInteger(page) && page > 0 && page <= 1000000 ? page - 1 : 0;
}

async function showItemRoute(id) {
  setWorkspace('collection');
  const generation = ++libraryGeneration;
  stage.replaceChildren(el('p', 'muted', 'Loading item…'));
  try {
    const data = await api(`/api/items/${encodeURIComponent(id)}`);
    if (generation !== libraryGeneration) return;
    openItemDetail(data.item || data, {});
  } catch (error) { if (generation === libraryGeneration) renderError(error); }
}

function restoreView() {
  // A response from a view we just left must not replace the restored screen.
  ++requestGeneration;
  if (activeController) { activeController.abort(); activeController = null; clearWorking(); }
  if (catalogModal.open) catalogModal.close();
  if (imageModal.open) imageModal.close();
  const params = new URLSearchParams(location.search);
  const parts = location.pathname.split('/').filter(Boolean);
  restoringView = true;
  try {
    if (!parts.length && params.get('identify')) {
      updateViewURL(`/identify/${encodeURIComponent(params.get('identify'))}`, {}, true);
      return openIdentification(params.get('identify'));
    }
    switch (parts[0] || '') {
      case 'pending': return showIntake(routePage(params));
      case 'review': return showReview();
      case 'admin': return showAdmin();
      case 'capture': return showCapture(params.get('category') || '');
      case 'identify': return openIdentification(decodeURIComponent(parts[1]));
      case 'items': return showItemRoute(decodeURIComponent(parts[1]));
      case 'canonical': return openCanonical(decodeURIComponent(parts[1]));
      default:
        libraryFilters = Object.fromEntries(['q', 'category_id', 'tag', 'location'].map(key => [key, params.get(key) || '']));
        return showLibrary(routePage(params));
    }
  } finally { restoringView = false; }
}

window.addEventListener('popstate', () => restoreView());
const initialMessage = new URLSearchParams(location.search).get('message');
if (initialMessage) {
  updateViewURL('/', {}, true);
  input.value = initialMessage;
  form.requestSubmit();
} else {
  restoreView();
}
