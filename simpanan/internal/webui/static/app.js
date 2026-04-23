// simpanan webui — vanilla JS app.
//
// State model:
//   files: Map<path, OpenFile>
//   active: string (the active path, "" if none)
//
// Server is canonical. The browser maintains a local mirror, sends
// edits as POST /api/files/edit (debounced), and listens to SSE for
// updates from itself + other tabs.

const editEl = document.getElementById("editor");
const tabsEl = document.getElementById("tabs");
const activePathEl = document.getElementById("active-path");
const modifiedEl = document.getElementById("modified-indicator");
const cursorInfoEl = document.getElementById("cursor-info");
const connStatusEl = document.getElementById("connection-status");
const openBtn = document.getElementById("open-btn");
const saveBtn = document.getElementById("save-btn");
const runBtn = document.getElementById("run-btn");
const resultPanel = document.getElementById("result-panel");
const resultBody = document.getElementById("result-body");

const state = {
	files: new Map(),
	active: "",
	// Suppresses local input handler reactions when the textarea is
	// programmatically updated from a remote event. Without this we'd
	// loop on our own broadcasts.
	applyingRemote: false,
	editTimer: null,
	editDelayMs: 100,
};

// ---- HTTP helpers --------------------------------------------------

async function api(path, opts = {}) {
	const res = await fetch(path, {
		method: opts.method || "GET",
		headers: opts.body ? { "Content-Type": "application/json" } : undefined,
		body: opts.body ? JSON.stringify(opts.body) : undefined,
	});
	const text = await res.text();
	if (!res.ok) {
		let err = text;
		try { err = JSON.parse(text).error || text; } catch (_) {}
		throw new Error(err || res.statusText);
	}
	return text ? JSON.parse(text) : null;
}

// ---- Rendering -----------------------------------------------------

function renderTabs() {
	tabsEl.innerHTML = "";
	for (const f of state.files.values()) {
		const tab = document.createElement("div");
		tab.className = "tab";
		if (f.path === state.active) tab.classList.add("active");
		tab.dataset.path = f.path;

		const label = document.createElement("span");
		label.className = "tab-label";
		label.textContent = f.path.split("/").pop();
		label.title = f.path;
		tab.appendChild(label);

		const mod = document.createElement("span");
		mod.className = "tab-modified";
		mod.textContent = f.status === "modified" ? "●" : "";
		tab.appendChild(mod);

		const close = document.createElement("button");
		close.className = "tab-close";
		close.textContent = "×";
		close.title = "Close";
		close.addEventListener("click", (e) => {
			e.stopPropagation();
			closeFile(f.path);
		});
		tab.appendChild(close);

		tab.addEventListener("click", () => switchActive(f.path));
		tabsEl.appendChild(tab);
	}
}

function renderActive() {
	const f = state.files.get(state.active);
	if (!f) {
		editEl.value = "";
		editEl.disabled = true;
		runBtn.disabled = true;
		activePathEl.textContent = "no file open";
		modifiedEl.textContent = "";
		cursorInfoEl.textContent = "";
		return;
	}
	state.applyingRemote = true;
	if (editEl.value !== f.buffer_contents) {
		editEl.value = f.buffer_contents;
	}
	editEl.disabled = false;
	runBtn.disabled = false;
	state.applyingRemote = false;

	activePathEl.textContent = f.path;
	modifiedEl.textContent = f.status === "modified" ? "modified" : "";
	updateCursorInfo();
}

function updateCursorInfo() {
	const pos = editEl.selectionStart;
	cursorInfoEl.textContent = `cursor ${pos}`;
}

function setConnectionStatus(label, klass) {
	connStatusEl.textContent = label;
	connStatusEl.classList.remove("connected", "disconnected");
	if (klass) connStatusEl.classList.add(klass);
}

// ---- Local actions -------------------------------------------------

async function openFile() {
	const path = prompt("Path to .simp file:");
	if (!path) return;
	try {
		const f = await api("/api/files/open", { method: "POST", body: { path } });
		state.files.set(f.path, f);
		state.active = f.path;
		renderTabs();
		renderActive();
	} catch (err) {
		alert("Open failed: " + err.message);
	}
}

async function saveFile() {
	if (!state.active) return;
	try {
		const f = await api("/api/files/save", { method: "POST", body: { path: state.active } });
		state.files.set(f.path, f);
		renderTabs();
		renderActive();
	} catch (err) {
		alert("Save failed: " + err.message);
	}
}

async function closeFile(path) {
	try {
		await api("/api/files/close", { method: "POST", body: { path } });
		state.files.delete(path);
		if (state.active === path) {
			// Promote some other open file or clear.
			const next = state.files.keys().next().value || "";
			state.active = next;
		}
		renderTabs();
		renderActive();
	} catch (err) {
		alert("Close failed: " + err.message);
	}
}

async function switchActive(path) {
	try {
		await api("/api/files/switch-active", { method: "POST", body: { path } });
		state.active = path;
		renderTabs();
		renderActive();
	} catch (err) {
		alert("Switch failed: " + err.message);
	}
}

function scheduleEditPush() {
	if (state.applyingRemote) return;
	if (!state.active) return;
	clearTimeout(state.editTimer);
	state.editTimer = setTimeout(pushEdit, state.editDelayMs);
}

async function pushEdit() {
	const path = state.active;
	if (!path) return;
	const buffer_contents = editEl.value;
	const cursor_byte_offset = editEl.selectionStart;
	try {
		const f = await api("/api/files/edit", {
			method: "POST",
			body: { path, buffer_contents, cursor_byte_offset },
		});
		state.files.set(f.path, f);
		// We do NOT call renderActive here — that would clobber the
		// user's cursor mid-edit. Just update tab state for the modified
		// indicator.
		renderTabs();
		modifiedEl.textContent = f.status === "modified" ? "modified" : "";
	} catch (err) {
		console.warn("edit push failed", err);
	}
}

// ---- SSE event handling --------------------------------------------

function applyRemoteFile(f) {
	state.files.set(f.path, f);
	if (f.path === state.active) {
		// Only restamp the textarea if the remote content differs from
		// what's local — otherwise we'd reset cursor on our own echo.
		if (editEl.value !== f.buffer_contents) {
			const cursor = editEl.selectionStart;
			state.applyingRemote = true;
			editEl.value = f.buffer_contents;
			editEl.selectionStart = editEl.selectionEnd = Math.min(cursor, f.buffer_contents.length);
			state.applyingRemote = false;
		}
		modifiedEl.textContent = f.status === "modified" ? "modified" : "";
	}
	renderTabs();
}

function handleEvent(ev) {
	switch (ev.type) {
		case "buffer_updated":
		case "file_saved":
			applyRemoteFile(ev.payload);
			break;
		case "file_opened":
			state.files.set(ev.payload.path, ev.payload);
			renderTabs();
			break;
		case "file_closed":
			state.files.delete(ev.payload.path);
			if (state.active === ev.payload.path) {
				const next = state.files.keys().next().value || "";
				state.active = next;
			}
			renderTabs();
			renderActive();
			break;
		case "active_switched":
			state.active = ev.payload.path;
			renderTabs();
			renderActive();
			break;
	}
}

function connectEvents() {
	const es = new EventSource("/api/events");
	es.onopen = () => setConnectionStatus("connected", "connected");
	es.onerror = () => setConnectionStatus("disconnected", "disconnected");
	es.onmessage = (msg) => {
		try {
			handleEvent(JSON.parse(msg.data));
		} catch (err) {
			console.warn("bad event payload", err, msg.data);
		}
	};
}

// ---- Bootstrap -----------------------------------------------------

async function bootstrap() {
	try {
		const list = await api("/api/files");
		state.files = new Map(list.files.map((f) => [f.path, f]));
		state.active = list.active || "";
	} catch (err) {
		console.warn("initial state load failed", err);
	}
	renderTabs();
	renderActive();
	connectEvents();
}

editEl.addEventListener("input", scheduleEditPush);
editEl.addEventListener("keyup", updateCursorInfo);
editEl.addEventListener("click", updateCursorInfo);

window.addEventListener("keydown", (e) => {
	if ((e.ctrlKey || e.metaKey) && e.key === "s") {
		e.preventDefault();
		saveFile();
	}
	if ((e.ctrlKey || e.metaKey) && e.key === "o") {
		e.preventDefault();
		openFile();
	}
	if ((e.ctrlKey || e.metaKey) && e.key === "Enter") {
		e.preventDefault();
		runSelection();
	}
});

// ---- Execute -------------------------------------------------------

function getSelectionText() {
	if (editEl.disabled) return "";
	const start = editEl.selectionStart;
	const end = editEl.selectionEnd;
	if (start === end) return ""; // explicit selection required
	return editEl.value.slice(start, end);
}

function showResult(text) {
	resultBody.textContent = text;
	resultPanel.hidden = false;
}

async function runSelection() {
	const selection = getSelectionText();
	if (!selection.trim()) {
		showResult("Select one or more pipeline stages first.");
		return;
	}
	showResult("running…");
	try {
		const data = await api("/api/execute", {
			method: "POST",
			body: { selection },
		});
		showResult(data.result || "(empty result)");
	} catch (err) {
		showResult("Request failed: " + err.message);
	}
}

// ---- Autocomplete --------------------------------------------------

const suggestionsEl = document.getElementById("suggestions");

const TRIGGER_CHARS = new Set([".", "{", ">", "$"]);
const SUGGEST_DEBOUNCE_MS = 80;

let suggestTimer = null;
let suggestState = {
	open: false,
	items: [],
	selected: -1,
	// The (start, end) byte range of the prefix the suggestion will
	// replace when accepted. Computed at popup-open time and updated
	// as the user types into the prefix.
	prefixStart: 0,
	prefixEnd: 0,
};

// Mirror element used to measure the cursor's pixel coordinates inside
// the textarea. Created lazily.
let cursorMirror = null;

function ensureCursorMirror() {
	if (cursorMirror) return cursorMirror;
	cursorMirror = document.createElement("div");
	cursorMirror.style.cssText = `
		position:absolute; visibility:hidden; white-space:pre-wrap;
		word-wrap:break-word; overflow-wrap:break-word; top:0; left:0;
	`;
	document.body.appendChild(cursorMirror);
	return cursorMirror;
}

function caretCoords() {
	const m = ensureCursorMirror();
	const cs = window.getComputedStyle(editEl);
	for (const prop of [
		"font-family", "font-size", "font-weight", "line-height",
		"letter-spacing", "padding-top", "padding-right",
		"padding-bottom", "padding-left", "border-top-width",
		"border-right-width", "border-bottom-width", "border-left-width",
		"box-sizing", "white-space", "word-wrap", "tab-size",
	]) {
		m.style.setProperty(prop, cs.getPropertyValue(prop));
	}
	const rect = editEl.getBoundingClientRect();
	m.style.width = rect.width + "px";

	const pos = editEl.selectionStart;
	m.textContent = editEl.value.slice(0, pos);
	const marker = document.createElement("span");
	marker.textContent = "​";
	m.appendChild(marker);

	const offsetTop = marker.offsetTop;
	const offsetLeft = marker.offsetLeft;

	return {
		top: rect.top + offsetTop - editEl.scrollTop,
		left: rect.left + offsetLeft - editEl.scrollLeft,
	};
}

function closeSuggestions() {
	suggestState.open = false;
	suggestState.items = [];
	suggestState.selected = -1;
	suggestionsEl.hidden = true;
}

function renderSuggestions() {
	suggestionsEl.innerHTML = "";
	suggestState.items.forEach((s, i) => {
		const li = document.createElement("li");
		li.className = "suggestion" + (i === suggestState.selected ? " active" : "");
		const text = document.createElement("span");
		text.className = "sug-text";
		text.textContent = s.text;
		const kind = document.createElement("span");
		kind.className = "sug-kind";
		kind.textContent = s.kind;
		li.appendChild(text);
		li.appendChild(kind);
		li.addEventListener("mousedown", (e) => {
			// mousedown (not click) so we accept before the textarea
			// loses focus to the popup.
			e.preventDefault();
			acceptSuggestion(i);
		});
		suggestionsEl.appendChild(li);
	});
}

function positionSuggestions() {
	const { top, left } = caretCoords();
	const lineHeight = parseFloat(window.getComputedStyle(editEl).lineHeight) || 18;
	suggestionsEl.style.top = (top + lineHeight) + "px";
	suggestionsEl.style.left = left + "px";
}

function showSuggestions(items, prefixStart, prefixEnd) {
	if (!items || items.length === 0) {
		closeSuggestions();
		return;
	}
	suggestState.open = true;
	suggestState.items = items;
	suggestState.selected = 0;
	suggestState.prefixStart = prefixStart;
	suggestState.prefixEnd = prefixEnd;
	renderSuggestions();
	suggestionsEl.hidden = false;
	positionSuggestions();
}

function moveSelection(delta) {
	if (!suggestState.open || suggestState.items.length === 0) return;
	const n = suggestState.items.length;
	suggestState.selected = (suggestState.selected + delta + n) % n;
	renderSuggestions();
}

function acceptSuggestion(index) {
	const i = index ?? suggestState.selected;
	const item = suggestState.items[i];
	if (!item) return;
	const before = editEl.value.slice(0, suggestState.prefixStart);
	const after = editEl.value.slice(editEl.selectionStart);
	editEl.value = before + item.text + after;
	const newCursor = before.length + item.text.length;
	editEl.selectionStart = editEl.selectionEnd = newCursor;
	closeSuggestions();
	scheduleEditPush();
}

// Compute the byte offset where the current "word" prefix begins,
// matching the Go classifier's wordPrefixRe (identifier-with-dots).
function prefixStartOffset(text, cursor) {
	let i = cursor;
	while (i > 0) {
		const c = text.charCodeAt(i - 1);
		const ch = text[i - 1];
		const isIdent =
			(c >= 0x41 && c <= 0x5a) ||  // A-Z
			(c >= 0x61 && c <= 0x7a) ||  // a-z
			(c >= 0x30 && c <= 0x39) ||  // 0-9
			ch === "_" || ch === "." || ch === "$" || ch === "\\";
		if (!isIdent) break;
		i--;
	}
	return i;
}

async function fetchSuggestions(forced) {
	const buffer_text = editEl.value;
	const cursor_byte_offset = editEl.selectionStart;
	try {
		const res = await api("/api/suggest", {
			method: "POST",
			body: { buffer_text, cursor_byte_offset },
		});
		const items = res.suggestions || [];
		if (items.length === 0) {
			if (forced) closeSuggestions();
			else closeSuggestions();
			return;
		}
		const start = prefixStartOffset(buffer_text, cursor_byte_offset);
		showSuggestions(items, start, cursor_byte_offset);
	} catch (err) {
		closeSuggestions();
	}
}

function scheduleSuggestions(forced) {
	clearTimeout(suggestTimer);
	suggestTimer = setTimeout(() => fetchSuggestions(forced), SUGGEST_DEBOUNCE_MS);
}

editEl.addEventListener("keydown", (e) => {
	if (suggestState.open) {
		if (e.key === "ArrowDown") { e.preventDefault(); moveSelection(1); return; }
		if (e.key === "ArrowUp")   { e.preventDefault(); moveSelection(-1); return; }
		if (e.key === "Enter" || e.key === "Tab") {
			e.preventDefault();
			acceptSuggestion();
			return;
		}
		if (e.key === "Escape") {
			e.preventDefault();
			closeSuggestions();
			return;
		}
	}
});

editEl.addEventListener("input", () => {
	if (state.applyingRemote) return;
	// Trigger characters or any non-whitespace keystroke kicks off a
	// fresh suggest. Whitespace closes a popup if one is open.
	const last = editEl.value[editEl.selectionStart - 1] || "";
	if (last === "" || /\s/.test(last)) {
		closeSuggestions();
		return;
	}
	if (TRIGGER_CHARS.has(last) || /[\w$]/.test(last)) {
		scheduleSuggestions(false);
	}
});

editEl.addEventListener("blur", closeSuggestions);
editEl.addEventListener("scroll", () => {
	if (suggestState.open) positionSuggestions();
});

openBtn.addEventListener("click", openFile);
saveBtn.addEventListener("click", saveFile);
runBtn.addEventListener("click", runSelection);

// ---- Connections popup ---------------------------------------------

const connectionsBtn = document.getElementById("connections-btn");
const modalBackdrop = document.getElementById("modal-backdrop");
const modalCloseBtn = document.getElementById("modal-close");
const connectionsListEl = document.getElementById("connections-list");
const addConnForm = document.getElementById("add-connection-form");
const connLabelInput = document.getElementById("conn-label");
const connUriInput = document.getElementById("conn-uri");
const connErrorEl = document.getElementById("conn-error");

async function refreshConnections() {
	try {
		const data = await api("/api/connections");
		renderConnectionsList(data.connections || []);
	} catch (err) {
		connErrorEl.textContent = "Failed to load connections: " + err.message;
	}
}

function renderConnectionsList(conns) {
	connectionsListEl.innerHTML = "";
	if (conns.length === 0) {
		const empty = document.createElement("li");
		empty.textContent = "No connections registered yet.";
		empty.style.color = "var(--fg-muted)";
		empty.style.justifyContent = "center";
		connectionsListEl.appendChild(empty);
		return;
	}
	for (const c of conns) {
		const li = document.createElement("li");

		const text = document.createElement("div");
		text.className = "conn-text";
		const label = document.createElement("span");
		label.className = "conn-label";
		label.textContent = c.label;
		const uri = document.createElement("span");
		uri.className = "conn-uri";
		uri.textContent = c.uri;
		text.appendChild(label);
		text.appendChild(uri);
		li.appendChild(text);

		const del = document.createElement("button");
		del.className = "conn-delete";
		del.type = "button";
		del.textContent = "Delete";
		del.addEventListener("click", () => deleteConnection(c.label));
		li.appendChild(del);

		connectionsListEl.appendChild(li);
	}
}

async function deleteConnection(label) {
	connErrorEl.textContent = "";
	try {
		const res = await fetch("/api/connections", {
			method: "DELETE",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify({ label }),
		});
		if (!res.ok) {
			const text = await res.text();
			let msg = text;
			try { msg = JSON.parse(text).error || text; } catch (_) {}
			connErrorEl.textContent = msg;
			return;
		}
		await refreshConnections();
	} catch (err) {
		connErrorEl.textContent = err.message;
	}
}

addConnForm.addEventListener("submit", async (e) => {
	e.preventDefault();
	connErrorEl.textContent = "";
	const label = connLabelInput.value.trim();
	const uri = connUriInput.value.trim();
	try {
		await api("/api/connections", { method: "POST", body: { label, uri } });
		connLabelInput.value = "";
		connUriInput.value = "";
		await refreshConnections();
	} catch (err) {
		connErrorEl.textContent = err.message;
	}
});

function openConnectionsModal() {
	connErrorEl.textContent = "";
	modalBackdrop.hidden = false;
	refreshConnections();
}

function closeConnectionsModal() {
	modalBackdrop.hidden = true;
}

connectionsBtn.addEventListener("click", openConnectionsModal);
modalCloseBtn.addEventListener("click", closeConnectionsModal);
modalBackdrop.addEventListener("click", (e) => {
	if (e.target === modalBackdrop) closeConnectionsModal();
});
window.addEventListener("keydown", (e) => {
	if (e.key === "Escape" && !modalBackdrop.hidden) closeConnectionsModal();
});

bootstrap();
