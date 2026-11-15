// Frontend del catálogo: llama a /api en el mismo host (el Ingress enruta /api a la API).
"use strict";

const HISTORY_SIZE = 30;
const POLL_MS = 2000;
const history = [];

const $ = (id) => document.getElementById(id);

function formatPrice(cents) {
  return new Intl.NumberFormat("es-PE", { style: "currency", currency: "USD" }).format(cents / 100);
}

async function getJSON(path) {
  const response = await fetch(path, { cache: "no-store" });
  if (!response.ok) throw new Error(`${path} respondió ${response.status}`);
  return response.json();
}

function renderProducts(products) {
  const list = $("products");
  list.replaceChildren();
  for (const product of products) {
    const item = document.createElement("li");
    item.className = "card";
    const name = document.createElement("h3");
    name.textContent = product.name;
    const description = document.createElement("p");
    description.textContent = product.description;
    const price = document.createElement("p");
    price.className = "price";
    price.textContent = formatPrice(product.price_cents);
    item.append(name, description, price);
    if (typeof product.stock === "number") {
      const stock = document.createElement("p");
      stock.className = "stock";
      stock.textContent = `Stock: ${product.stock}`;
      item.append(stock);
    }
    list.append(item);
  }
  $("status").textContent = `${products.length} productos`;
}

function renderVersion(info) {
  $("api-version").textContent = info.version;
  document.documentElement.style.setProperty("--version-color", info.color);
  history.push(info);
  if (history.length > HISTORY_SIZE) history.shift();
  const list = $("history");
  list.replaceChildren(...history.map((entry) => {
    const dot = document.createElement("li");
    dot.style.background = entry.color;
    dot.title = entry.version;
    return dot;
  }));
}

async function loadProducts() {
  try {
    renderProducts(await getJSON("/api/products"));
  } catch (error) {
    $("status").textContent = `No se pudo cargar el catálogo: ${error.message}`;
  }
}

async function pollVersion() {
  try {
    renderVersion(await getJSON("/api/version"));
  } catch (error) {
    $("api-version").textContent = "sin respuesta";
  }
}

$("frontend-version").textContent = window.CATALOGO_FRONTEND_VERSION || "dev";
loadProducts();
pollVersion();
setInterval(pollVersion, POLL_MS);
