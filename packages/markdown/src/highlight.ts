import hljs from "highlight.js/lib/core";
import bash from "highlight.js/lib/languages/bash";
import c from "highlight.js/lib/languages/c";
import cpp from "highlight.js/lib/languages/cpp";
import csharp from "highlight.js/lib/languages/csharp";
import css from "highlight.js/lib/languages/css";
import diff from "highlight.js/lib/languages/diff";
import dockerfile from "highlight.js/lib/languages/dockerfile";
import go from "highlight.js/lib/languages/go";
import java from "highlight.js/lib/languages/java";
import javascript from "highlight.js/lib/languages/javascript";
import json from "highlight.js/lib/languages/json";
import kotlin from "highlight.js/lib/languages/kotlin";
import markdown from "highlight.js/lib/languages/markdown";
import php from "highlight.js/lib/languages/php";
import python from "highlight.js/lib/languages/python";
import ruby from "highlight.js/lib/languages/ruby";
import rust from "highlight.js/lib/languages/rust";
import shell from "highlight.js/lib/languages/shell";
import sql from "highlight.js/lib/languages/sql";
import swift from "highlight.js/lib/languages/swift";
import typescript from "highlight.js/lib/languages/typescript";
import xml from "highlight.js/lib/languages/xml";
import yaml from "highlight.js/lib/languages/yaml";

/*
 * The languages issue bodies are coloured in: the catalog's common ones, plus
 * the formats people paste (shell sessions, diffs, logs as plain text). A
 * block in any other language stays plain. Its own chunk: loaded by
 * highlightCode only when a body has code.
 */
const languages = {
  bash, c, cpp, csharp, css, diff, dockerfile, go, java, javascript, json, kotlin,
  markdown, php, python, ruby, rust, shell, sql, swift, typescript, xml, yaml,
};
for (const [name, lang] of Object.entries(languages)) hljs.registerLanguage(name, lang);
hljs.configure({ ignoreUnescapedHTML: true });

export default hljs;
