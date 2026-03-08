import { Languages } from "lucide-react";
import { Button } from "@/ui/components/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/ui/components/dropdown-menu";
import { getLocale, setLocale, locales } from "@/i18n/runtime";
import { m } from "@/i18n/messages";

const LOCALE_LABELS: Record<string, () => string> = {
  en: m.language_en,
  de: m.language_de,
};

export function LanguageToggle() {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label={m.language()}>
          <Languages className="h-4 w-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {locales.map((locale) => (
          <DropdownMenuItem
            key={locale}
            onClick={() => setLocale(locale as "en" | "de")}
            data-active={getLocale() === locale}
            className="data-[active=true]:font-semibold"
          >
            {(LOCALE_LABELS[locale] ?? (() => locale))()}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
