import { ThemeType } from "grommet/themes";

export const theme: ThemeType | any = {
    // "name": "my theme",
    "rounding": 4,
    "spacing": 24,
    "defaultMode": "light",
    "global": {
        "colors": {
            "brand": "#FDC953",
            "background": {
                "dark": "#080300",
                "light": "#FFFFFF"
            },
            "background-strong": {
                "dark": "#000000",
                "light": "#FFFFFF"
            },
            "background-weak": {
                "dark": "#6F6B68",
                "light": "#E7E7E7"
            },
            "background-xweak": {
                "dark": "#66666699",
                "light": "#CCCCCC90"
            },
            "text": {
                "dark": "#EEEEEE",
                "light": "#080300"
            },
            "text-strong": {
                "dark": "#FFFFFF",
                "light": "#000000"
            },
            "text-weak": {
                "dark": "#CCCCCC",
                "light": "#444444"
            },
            "text-xweak": {
                "dark": "#999999",
                "light": "#666666"
            },
            "border": "background-xweak",
            "control": "brand",
            "active-background": "background-weak",
            "active-text": "text-strong",
            "selected-background": "background-strong",
            "selected-text": "text-strong",
            "status-critical": "#F75E60",
            "status-warning": "#FDC854",
            "status-ok": "#2EC990",
            "status-unknown": "#CCCCCC",
            "status-disabled": "#CCCCCC"
        },
        "font": {
            "family": "IBM Plex Sans, Helvetica, sans-serif",
        },
    }
};
