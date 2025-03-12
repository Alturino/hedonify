import http from "k6/http";
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

export default function () {
  randomSeed(999_999_999);

  // randomize user
  const userRandomIndex = Math.floor(Math.random() * users.length);
  const user = users[userRandomIndex];
  const loginResponse = http
    .post("http://localhost/users/login", JSON.stringify(user), {})
    .json();
  const isLoginSuccess = check(loginResponse, {
    "Success login user": (r) => r?.statusCode === 200,
  });
  if (!isLoginSuccess) {
    console.error(`loginResponse=${JSON.stringify(loginResponse)}`);
    fail("Failed login user");
  }
  console.log(`loginResponse=${JSON.stringify(loginResponse)}`);
  const token = loginResponse.data.token;

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
  const insertCartResponse = http
    .post("http://localhost/carts", JSON.stringify({ cart_items: cartItems }), {
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
    })
    .json();
  const isInsertCartSuccess = check(insertCartResponse, {
    "Success insert cart": (r) => r?.statusCode === 200,
  });
  if (!isInsertCartSuccess) {
    console.error(`insertCartResponse=${JSON.stringify(insertCartResponse)}`);
    fail("Failed insert cart");
  }
  console.log(`insertCartResponse=${JSON.stringify(insertCartResponse)}`);

  // checkout cart
  const cartId = insertCartResponse.data.cart.id;
  const checkoutResponse = http
    .post(`http://localhost/carts/${cartId}/checkout`, JSON.stringify({}), {
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
    })
    .json();
  const isCheckoutSuccess = check(checkoutResponse, {
    "Success checkout cart": (r) => r?.statusCode === 200,
  });
  if (!isCheckoutSuccess) {
    counterOrderFail.add(1);
    console.error(`checkoutResponse=${JSON.stringify(checkoutResponse)}`);
    fail("Failed checkout");
  }
  console.log(`checkoutResponse=${JSON.stringify(checkoutResponse)}`);
  counterOrderSuccess.add(1);
}
