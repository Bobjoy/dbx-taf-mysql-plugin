import { EditorView, keymap, lineNumbers, highlightActiveLine, highlightActiveLineGutter, placeholder, tooltips } from "@codemirror/view";
import { EditorState } from "@codemirror/state";
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import { sql, MySQL } from "@codemirror/lang-sql";
import { autocompletion, closeBrackets, closeBracketsKeymap, completionKeymap } from "@codemirror/autocomplete";
import { bracketMatching, HighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { tags as t } from "@lezer/highlight";

// 用 CSS 系统色，明暗两套 color-scheme 自适应，不需要分主题
const theme = EditorView.theme({
  "&": { backgroundColor: "transparent", color: "CanvasText", fontSize: "13px" },
  "&.cm-focused": { outline: "none" },
  ".cm-content": { fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace", padding: "6px 0", caretColor: "CanvasText" },
  ".cm-scroller": { overflow: "auto" },
  ".cm-gutters": { backgroundColor: "transparent", color: "color-mix(in srgb, CanvasText 40%, transparent)", border: "none" },
  ".cm-activeLine": { backgroundColor: "color-mix(in srgb, CanvasText 5%, transparent)" },
  ".cm-activeLineGutter": { backgroundColor: "transparent", color: "CanvasText" },
  ".cm-selectionBackground, &.cm-focused .cm-selectionBackground": { backgroundColor: "color-mix(in srgb, #6d5dfc 26%, transparent)" },
});

const highlight = HighlightStyle.define([
  { tag: t.keyword, color: "#8250df" },
  { tag: [t.operator, t.punctuation], color: "color-mix(in srgb, CanvasText 75%, transparent)" },
  { tag: t.string, color: "#0a7d55" },
  { tag: t.number, color: "#b35900" },
  { tag: t.comment, color: "color-mix(in srgb, CanvasText 45%, transparent)", fontStyle: "italic" },
  { tag: t.typeName, color: "#8250df" },
  { tag: t.propertyName, color: "#0550ae" },
]);

function completeSource(complete) {
  return async (context) => {
    const word = context.matchBefore(/[`"]?\w*[`"]?/);
    if (!word && !context.explicit) return null;
    const prefixRaw = word ? word.text : "";
    const prefix = prefixRaw.replace(/[`"]/g, "");
    const before = context.state.doc.sliceString(0, context.pos);
    const options = await complete(prefix, before);
    if (!options.length) return null;
    return {
      from: context.pos - prefixRaw.length,
      options: options.map((o) => ({ label: o.label, detail: o.detail, type: o.type || "text" })),
      validFor: /^[`"]?\w*[`"]?$/,
    };
  };
}

export function createEditor(parent, { onRun, onComplete, initialValue = "" }) {
  const view = new EditorView({
    parent,
    state: EditorState.create({
      doc: initialValue,
      extensions: [
        lineNumbers(),
        highlightActiveLine(),
        highlightActiveLineGutter(),
        history(),
        bracketMatching(),
        closeBrackets(),
        autocompletion({ override: [completeSource(onComplete)], icons: true }),
        // 弹层挂 body：避开编辑器容器 overflow 裁切
        tooltips({ parent: document.body }),
        sql({ dialect: MySQL }),
        syntaxHighlighting(highlight),
        placeholder("单条 SQL；输入表名/列名自动补全"),
        theme,
        keymap.of([
          { key: "Mod-Enter", run: () => { onRun(); return true; } },
          ...closeBracketsKeymap,
          ...defaultKeymap,
          ...completionKeymap,
          ...historyKeymap,
        ]),
      ],
    }),
  });
  return {
    view,
    get value() { return view.state.doc.toString(); },
    set value(text) {
      view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: text } });
    },
    insertAtCursor(text) {
      const { from, to } = view.state.selection.main;
      view.dispatch({ changes: { from, to, insert: text }, selection: { anchor: from + text.length } });
      view.focus();
    },
    focus() { view.focus(); },
  };
}
