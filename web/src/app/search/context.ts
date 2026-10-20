import { createContext, useContext } from "react";

/** The search overlay: whether it is open, and opening it (the nav, "/" and Ctrl/⌘K). */
export const SearchContext = createContext<{ isOpen: boolean; open: () => void }>({ isOpen: false, open: () => {} });

export const useSearchOverlay = () => useContext(SearchContext);
