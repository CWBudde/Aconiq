import { Languages } from "lucide-react";
import { Button } from "@/ui/components/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/ui/components/dropdown-menu";
import { changeLocale, useLocale } from "@/locale";
import { locales } from "@/i18n/runtime";
import { m } from "@/i18n/messages";

type Locale = (typeof locales)[number];

const LOCALE_LABELS: Record<Locale, () => string> = {
  en: m.language_en,
  de: m.language_de,
};

export function LanguageToggle() {
  const active = useLocale();

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
            onClick={() => {
              changeLocale(locale);
            }}
            data-active={active === locale}
            className="data-[active=true]:font-semibold"
          >
            {LOCALE_LABELS[locale]()}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
