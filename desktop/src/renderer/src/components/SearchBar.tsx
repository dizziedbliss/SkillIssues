import { useApp } from "../state/AppState";
import { SearchIcon } from "../lib/icons";

export function SearchBar() {
  const app = useApp();

  return (
    <section className="searchbar">
      <span className="search-icon">
        <SearchIcon />
      </span>
      <input
        value={app.searchInput}
        onChange={(e) => app.setSearchInput(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") app.submitSearch();
        }}
        placeholder="Search challenges by topic, language, or repository..."
      />
      <button type="button" onClick={app.clearSearch}>
        clear
      </button>
      <button type="button" id="search-button" onClick={app.submitSearch}>
        Search
      </button>
    </section>
  );
}
