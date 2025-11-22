/** @type {import('tailwindcss').Config} */
module.exports = {
    content: [
        "./src/**/*.{go,templ}",
        "./templates/**/*.templ",
    ],
    theme: {
        extend: {},
    },
    plugins: [
        require("daisyui"),
    ],
    daisyui: {
        themes: ["light", "dark", "cupcake"],
    },
}

