// simpanan webui — CodeMirror 6 + vanilla JS app.
//
// State:
//   files: Map<path, OpenFile>      (server-canonical mirror)
//   active: string                   (active path, "" if none)
//   view: EditorView | null          (CM6 instance, null until first file open)
//
// Server is canonical. The browser maintains a local mirror, sends edits
// as POST /api/files/edit (debounced), and listens to /api/events SSE
// for updates from itself + other tabs.
//
// CM6 is loaded as ES modules from esm.sh — no bundler, no node
// toolchain. First-load requires internet; subsequent loads are
// browser-cached. For strict offline use, swap the imports for a
// vendored bundle.

import { EditorView, basicSetup } from "https://esm.sh/codemirror@6.0.1";
import { keymap } from "https://esm.sh/@codemirror/view@6.26.3";
import { Annotation } from "https://esm.sh/@codemirror/state@6.4.1";
import { autocompletion } from "https://esm.sh/@codemirror/autocomplete@6.16.0";
import { defaultKeymap, indentWithTab } from "https://esm.sh/@codemirror/commands@6.5.0";
import { simpStreamLanguage } from "/static/simp_lang.js";

const editorContainer = document.getElementById("editor");
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

// Loop guard: when we apply an edit received from SSE, we annotate the
// CM transaction so our own change handler skips the broadcast.
const remoteAnnotation = Annotation.define();

const state = {
	files: new Map(),
	active: "",
	view: null,
	editTimer: null,
	editDelayMs: 100,
	// Live label → connection-type map consumed by the simp language
	// extension. Wrapped in {value: …} so the StreamLanguage parser
	// holds a stable reference and always sees the latest map.
	connTypes: new Map(),
};
const connTypesRef = { value: state.connTypes };

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

// ---- CodeMirror integration ---------------------------------------

function ensureView(initialContents) {
	if (state.view) return state.view;
	editorContainer.removeAttribute("data-empty");
	editorContainer.innerHTML = "";

	state.view = new EditorView({
		parent: editorContainer,
		doc: initialContents || "",
		extensions: [
			basicSetup,
			keymap.of([indentWithTab, ...defaultKeymap]),
			simpStreamLanguage(connTypesRef),
			autocompletion({
				override: [simpCompletion],
				closeOnBlur: true,
				activateOnTyping: true,
			}),
			EditorView.theme({
				"&": { height: "100%", backgroundColor: "var(--bg)" },
			}),
			EditorView.updateListener.of(handleViewUpdate),
		],
	});
	return state.view;
}

function destroyView() {
	if (!state.view) return;
	state.view.destroy();
	state.view = null;
	editorContainer.innerHTML = "";
	editorContainer.setAttribute("data-empty", "true");
}

function getDocText() {
	return state.view ? state.view.state.doc.toString() : "";
}

function getCursorOffset() {
	return state.view ? state.view.state.selection.main.head : 0;
}

function getSelectionText() {
	if (!state.view) return "";
	const sel = state.view.state.selection.main;
	if (sel.empty) return "";
	return state.view.state.sliceDoc(sel.from, sel.to);
}

function setDocFromRemote(text) {
	if (!state.view) {
		ensureView(text);
		return;
	}
	state.view.dispatch({
		changes: { from: 0, to: state.view.state.doc.length, insert: text },
		annotations: remoteAnnotation.of(true),
	});
}

function handleViewUpdate(update) {
	if (update.selectionSet || update.docChanged) {
		updateCursorInfo();
	}
	if (!update.docChanged) return;
	// Skip self-loops on remote-applied transactions.
	const isRemote = update.transactions.some((tr) =>
		tr.annotation(remoteAnnotation) === true
	);
	if (isRemote) return;
	scheduleEditPush();
}

function updateCursorInfo() {
	cursorInfoEl.textContent = `cursor ${getCursorOffset()}`;
}

// ---- Suggestion source -------------------------------------------

const SUGGESTION_KIND_ICON = {
	connection_label: "module",
	sql_keyword: "keyword",
	database: "namespace",
	table: "class",
	column: "property",
	mongo_collection: "class",
	mongo_operation: "function",
	mongo_operator: "interface",
	mongo_field: "property",
	redis_command: "function",
	jq_operator: "function",
	jq_path: "variable",
};

async function simpCompletion(context) {
	if (!state.active) return null;

	const text = context.state.doc.toString();
	const cursor = context.pos;

	let res;
	try {
		res = await api("/api/suggest", {
			method: "POST",
			body: { buffer_text: text, cursor_byte_offset: cursor },
		});
	} catch (err) {
		return null;
	}
	const items = res.suggestions || [];
	if (items.length === 0) return null;

	// Match the Go classifier's wordPrefixRe so CM replaces the right
	// slice when accepting.
	const before = text.slice(0, cursor);
	let from = cursor;
	for (let i = cursor; i > 0; i--) {
		const c = before.charCodeAt(i - 1);
		const ch = before[i - 1];
		const isIdent =
			(c >= 0x41 && c <= 0x5a) ||
			(c >= 0x61 && c <= 0x7a) ||
			(c >= 0x30 && c <= 0x39) ||
			ch === "_" || ch === "." || ch === "$" || ch === "\\";
		if (!isIdent) break;
		from = i - 1;
	}

	return {
		from,
		options: items.map((s) => ({
			label: s.text,
			type: SUGGESTION_KIND_ICON[s.kind] || "text",
			detail: s.kind,
		})),
	};
}

// ---- Tabs / state mirror ------------------------------------------

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
		destroyView();
		runBtn.disabled = true;
		activePathEl.textContent = "no file open";
		modifiedEl.textContent = "";
		cursorInfoEl.textContent = "";
		return;
	}

	if (!state.view) {
		ensureView(f.buffer_contents);
	} else if (getDocText() !== f.buffer_contents) {
		setDocFromRemote(f.buffer_contents);
	}
	runBtn.disabled = false;
	activePathEl.textContent = f.path;
	modifiedEl.textContent = f.status === "modified" ? "modified" : "";
	updateCursorInfo();
	if (state.view) state.view.focus();
}

function setConnectionStatus(label, klass) {
	connStatusEl.textContent = label;
	connStatusEl.classList.remove("connected", "disconnected");
	if (klass) connStatusEl.classList.add(klass);
}

// ---- Local actions ------------------------------------------------

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
		modifiedEl.textContent = f.status === "modified" ? "modified" : "";
	} catch (err) {
		alert("Save failed: " + err.message);
	}
}

async function closeFile(path) {
	try {
		await api("/api/files/close", { method: "POST", body: { path } });
		state.files.delete(path);
		if (state.active === path) {
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
	if (!state.active) return;
	clearTimeout(state.editTimer);
	state.editTimer = setTimeout(pushEdit, state.editDelayMs);
}

async function pushEdit() {
	const path = state.active;
	if (!path) return;
	try {
		const f = await api("/api/files/edit", {
			method: "POST",
			body: {
				path,
				buffer_contents: getDocText(),
				cursor_byte_offset: getCursorOffset(),
			},
		});
		state.files.set(f.path, f);
		renderTabs();
		modifiedEl.textContent = f.status === "modified" ? "modified" : "";
	} catch (err) {
		console.warn("edit push failed", err);
	}
}

// ---- Execute ------------------------------------------------------

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

// ---- SSE event handling -------------------------------------------

function applyRemoteFile(f) {
	state.files.set(f.path, f);
	if (f.path === state.active) {
		if (getDocText() !== f.buffer_contents) {
			setDocFromRemote(f.buffer_contents);
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

// ---- Connections popup --------------------------------------------

const connectionsBtn = document.getElementById("connections-btn");
const modalBackdrop = document.getElementById("modal-backdrop");
const modalCloseBtn = document.getElementById("modal-close");
const connectionsListEl = document.getElementById("connections-list");
const addConnForm = document.getElementById("add-connection-form");
const connLabelInput = document.getElementById("conn-label");
const connUriInput = document.getElementById("conn-uri");
const connErrorEl = document.getElementById("conn-error");

// schemeToConnType mirrors common.URI.ConnType on the server: maps a
// URI scheme to the canonical connection type used by the schema cache
// and the syntax highlighter.
function schemeToConnType(scheme) {
	switch (scheme) {
		case "postgres":
		case "postgresql":
			return "postgres";
		case "mysql":
			return "mysql";
		case "mongodb":
		case "mongodb+srv":
			return "mongo";
		case "redis":
		case "rediss":
			return "redis";
	}
	return null;
}

function ingestConnections(conns) {
	state.connTypes = new Map();
	for (const c of conns) {
		const m = (c.uri || "").match(/^([a-z+]+):\/\//i);
		const ct = m ? schemeToConnType(m[1].toLowerCase()) : null;
		if (ct) state.connTypes.set(c.label, ct);
	}
	connTypesRef.value = state.connTypes;
}

async function refreshConnections() {
	try {
		const data = await api("/api/connections");
		const conns = data.connections || [];
		ingestConnections(conns);
		renderConnectionsList(conns);
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

// ---- Global key bindings + bootstrap -----------------------------

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
	if (e.key === "Escape" && !modalBackdrop.hidden) {
		closeConnectionsModal();
	}
});

openBtn.addEventListener("click", openFile);
saveBtn.addEventListener("click", saveFile);
runBtn.addEventListener("click", runSelection);

async function bootstrap() {
	// Load files and connections in parallel so the editor lights up
	// quickly even if either endpoint is slow.
	const filesP = api("/api/files").catch((err) => {
		console.warn("initial files load failed", err);
		return { files: [], active: "" };
	});
	const connsP = api("/api/connections").catch((err) => {
		console.warn("initial connections load failed", err);
		return { connections: [] };
	});
	const [list, connsResp] = await Promise.all([filesP, connsP]);
	state.files = new Map((list.files || []).map((f) => [f.path, f]));
	state.active = list.active || "";
	ingestConnections(connsResp.connections || []);

	renderTabs();
	renderActive();
	connectEvents();
}

bootstrap();
