document.addEventListener("DOMContentLoaded", function () {
    const toggleThemeBtn = document.createElement("button");
    toggleThemeBtn.innerText = "Toggle Dark Mode";
    toggleThemeBtn.style.position = "fixed";
    toggleThemeBtn.style.top = "10px";
    toggleThemeBtn.style.right = "10px";
    toggleThemeBtn.style.padding = "5px 10px";
    toggleThemeBtn.style.cursor = "pointer";
    document.body.appendChild(toggleThemeBtn);

    function setTheme(theme) {
        document.documentElement.setAttribute("data-theme", theme);
        localStorage.setItem("theme", theme);
    }

    function toggleTheme() {
        const currentTheme = localStorage.getItem("theme");
        if (currentTheme === "dark") {
            setTheme("light");
        } else {
            setTheme("dark");
        }
    }

    toggleThemeBtn.addEventListener("click", toggleTheme);

    const savedTheme = localStorage.getItem("theme") || (window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");
    setTheme(savedTheme);
});
