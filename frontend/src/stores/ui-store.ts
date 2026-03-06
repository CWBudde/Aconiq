import { create } from "zustand";

interface UIState {
  /** Currently active navigation item */
  activeNav: string;
  setActiveNav: (nav: string) => void;

  /** Whether a run is in progress (guards navigation away) */
  runInProgress: boolean;
  setRunInProgress: (running: boolean) => void;
}

export const useUIStore = create<UIState>((set) => ({
  activeNav: "map",
  setActiveNav: (nav) => {
    set({ activeNav: nav });
  },

  runInProgress: false,
  setRunInProgress: (running) => {
    set({ runInProgress: running });
  },
}));
