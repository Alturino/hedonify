import sql from "k6/x/sql";
import http from "k6/http";
import driver from "k6/x/sql/driver/postgres";
import { check, fail, randomSeed } from "k6";
import { Options } from "k6/options";
import { SharedArray } from "k6/data";
import { Counter } from "k6/metrics";

const minQuantity = 1;
const maxQuantity = 10;
const minOrderItems = 1;
const maxOrderItems = 5;

export const options: Options = {
  scenarios: {
    order_creations: {
      executor: "per-vu-iterations",
      vus: 20,
      iterations: 5,
      maxDuration: "30s",
    },
  },
};

const users = new SharedArray("users", () => {
  return JSON.parse(open("../seed/users.seed.json"));
});

const products = new SharedArray("products", () => {
  return JSON.parse(open("../seed/products.seed.json"));
});

const counterOrderSuccess = new Counter("order_success");
const counterOrderFail = new Counter("order_fail");

const db = sql.open(
  driver,
  "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable",
);
const productSeedDb = open("../seed/products.seed.sql");
const userSeedDb = open("../seed/users.seed.sql");

export function setup() {
  db.exec(productSeedDb);
  db.exec(userSeedDb);
}

export function teardown() {
  db.close();
}

export default function () {
  randomSeed(999_999_999);

  // randomize user
  const userRandomIndex = Math.floor(Math.random() * users.length);
  const user = users[userRandomIndex];
  const loginReq = http.post(
    "http://localhost/users/login",
    JSON.stringify(user),
    {},
  );
  console.log(`loginReq=${JSON.stringify(loginReq)}`);
  const isLoginSuccess = check(loginReq.json(), {
    "Success login user": (r) => r?.statusCode === 200,
  });
  if (!isLoginSuccess) {
    console.error(`loginRes=${JSON.stringify(loginReq.json())}`);
    fail("Failed login user");
  }
  console.log(`loginRes=${JSON.stringify(loginReq.json())}`);
  const loginRes = loginReq.json();
  const token = loginRes.data.token;

  // randomize product selection
  const cartItems = [];
  const randomItemCount = Math.floor(
    Math.random() * (maxOrderItems - minOrderItems) + minOrderItems,
  );
  for (let i = 0; i < randomItemCount; i++) {
    const product = products[Math.floor(Math.random() * products.length)];
    const newProduct = {
      price: product.price,
      product_id: product.id,
      quantity: Math.floor(
        Math.random() * (maxQuantity - minQuantity) + minQuantity,
      ),
    };
    cartItems.push(newProduct);
  }
  console.log(`cartItems=${JSON.stringify(cartItems)}`);

  // insert cart
  const insertCartReq = http.post(
    "http://localhost/carts",
    JSON.stringify({ cart_items: cartItems }),
    {
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
    },
  );
  console.log(`insertCartReq=${JSON.stringify(insertCartReq)}`);
  const isInsertCartSuccess = check(insertCartReq.json(), {
    "Success insert cart": (r) => r?.statusCode === 200,
  });
  if (!isInsertCartSuccess) {
    console.error(`insertCartResponse=${JSON.stringify(insertCartReq)}`);
    fail("Failed insert cart");
  }
  const insertCartRes = insertCartReq.json();
  console.log(`insertCartResponse=${JSON.stringify(insertCartRes)}`);

  // checkout cart
  const cartId = insertCartReq.json().data.cart.id;
  const checkoutReq = http.post(
    `http://localhost/carts/${cartId}/checkout`,
    JSON.stringify({}),
    {
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
    },
  );
  console.log(`checkoutReq=${JSON.stringify(checkoutReq)}`);
  const isCheckoutSuccess = check(checkoutReq.json(), {
    "Success checkout cart": (r) => r?.statusCode === 200,
  });
  if (!isCheckoutSuccess) {
    counterOrderFail.add(1);
    console.error(`checkoutResponse=${JSON.stringify(checkoutReq)}`);
    fail("Failed checkout");
  }
  console.log(`checkoutResponse=${JSON.stringify(checkoutReq)}`);
  counterOrderSuccess.add(1);
}
