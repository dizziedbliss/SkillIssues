// Ported from web/src/App.tsx — desktop's app.js never had this (it shows
// language as plain text), so this is a visual addition there, not a port.
const languageIcons: Record<string, string> = {
  Python: "python",
  TypeScript: "typescript",
  JavaScript: "javascript",
  HTML: "html5",
  CSS: "css3",
};

const languageLogoBase =
  "https://raw.githubusercontent.com/abranhe/programming-languages-logos/master/src";

export function LanguageMark({ name }: { name: string }) {
  const icon = languageIcons[name];
  return icon ? (
    <img
      className="language-mark"
      src={`${languageLogoBase}/${icon}/${icon}.svg`}
      alt={`${name} logo`}
      onError={(event) => {
        event.currentTarget.style.display = "none";
      }}
    />
  ) : (
    <span className="language-mark text-mark">{name.slice(0, 2).toUpperCase()}</span>
  );
}
